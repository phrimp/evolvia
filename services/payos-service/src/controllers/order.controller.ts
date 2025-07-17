import { Elysia } from "elysia";
import payOS from "../utils/payos";
import { rabbitMQService } from "../utils/rabbitmq";
import { PaymentMessage } from "../handlers/payment.handler";
import { mongoDBHandler } from "../handlers/mongodb.handler";

export const orderController = new Elysia({ prefix: "/order" })
  .post("/create", async ({ body }) => {
    console.log("Order creation request received:", JSON.stringify(body, null, 2));
    
    const { userId, description, returnUrl, cancelUrl, amount, subscriptionId, subscriptionID } = body as {
      userId: string;
      description: string;
      returnUrl: string;
      cancelUrl: string;
      amount: number;
      subscriptionId?: string;
      subscriptionID?: string;
    };

    // Handle both subscriptionId and subscriptionID
    const finalSubscriptionId = subscriptionId || subscriptionID || "";

    // Debug: Log extracted values
    console.log("DEBUG - Extracted values:");
    console.log("  - userId:", userId);
    console.log("  - amount:", amount);
    console.log("  - subscriptionId (from body):", subscriptionId);
    console.log("  - subscriptionID (from body):", subscriptionID);
    console.log("  - finalSubscriptionId:", finalSubscriptionId);

    const orderData = {
      orderCode: Number(String(new Date().getTime()).slice(-6)),
      amount,
      description,
      cancelUrl,
      returnUrl,
      expiredAt: Math.floor((Date.now() + 60 * 15 * 1000) / 1000), // 15 minute from now (Unix timestamp)
    };

    console.log("Order data prepared:", orderData);

    try {
      // Create PayOS payment link
      console.log("Creating PayOS payment link...");
      const paymentLinkRes = await payOS.createPaymentLink(orderData);
      console.log("PayOS payment link created:", paymentLinkRes);

      // Save transaction to MongoDB
      console.log("Saving transaction to MongoDB...");
      
      const transactionData: any = {
        userId,
        orderCode: orderData.orderCode.toString(),
        amount: orderData.amount,
        description: orderData.description,
        checkoutUrl: paymentLinkRes.checkoutUrl,
        subscriptionID: finalSubscriptionId,
        returnUrl: returnUrl,
        cancelUrl: cancelUrl,
      };
      
      console.log("DEBUG - Transaction data being saved:");
      console.log("  - subscriptionID value:", transactionData.subscriptionID);
      console.log("  - Full transaction data:", JSON.stringify(transactionData, null, 2));
      
      try {
        console.log("DEBUG - About to save transaction to MongoDB...");
        const savedTransaction = await mongoDBHandler.createTransaction(transactionData);
        console.log("Transaction saved to MongoDB successfully");
        console.log("DEBUG - Saved transaction subscriptionID:", savedTransaction.subscriptionID);
        
      } catch (mongoError) {
        console.error("Error saving to MongoDB:", mongoError);
        throw mongoError;
      }

      // Publish order creation event to internal queue
      await rabbitMQService.publishToQueue("order.updates", {
        type: "ORDER_CREATED",
        orderCode: orderData.orderCode.toString(),
        timestamp: new Date().toISOString(),
        data: {
          userId,
          amount: orderData.amount,
          description: orderData.description,
          checkoutUrl: paymentLinkRes.checkoutUrl,
          subscriptionId: finalSubscriptionId,
        },
      });

      console.log("DEBUG - Final response data:");
      console.log("  - orderCode:", paymentLinkRes.orderCode);
      console.log("  - amount:", paymentLinkRes.amount);
      console.log("  - subscriptionId:", finalSubscriptionId);

      return {
        error: 0,
        message: "Success",
        data: {
          bin: paymentLinkRes.bin,
          checkoutUrl: paymentLinkRes.checkoutUrl,
          accountNumber: paymentLinkRes.accountNumber,
          accountName: paymentLinkRes.accountName,
          amount: paymentLinkRes.amount,
          description: paymentLinkRes.description,
          orderCode: paymentLinkRes.orderCode,
          qrCode: paymentLinkRes.qrCode,
          subscriptionId: finalSubscriptionId,
        },
      };
    } catch (error) {
      console.error("PayOS error details:", error);

      // Publish order creation failure to internal queue
      await rabbitMQService.publishToQueue("payment.failed", {
        type: "ORDER_CREATION_FAILED",
        orderCode: orderData.orderCode.toString(),
        error: error instanceof Error ? error.message : String(error),
        timestamp: new Date().toISOString(),
      });

      return {
        error: -1,
        message: "fail",
        data: null,
      };
    }
  })
  .get("/user/:userId", async ({ params: { userId } }) => {
    try {
      console.log("Getting orders for user:", userId);
      
      const transactions = await mongoDBHandler.getTransactionsByUserId(userId);
      
      return {
        error: 0,
        message: "Success",
        data: transactions,
      };
    } catch (error) {
      console.error("Error getting user orders:", error);
      return {
        error: -1,
        message: "Failed to get user orders",
        data: null,
      };
    }
  })
  .get("/:orderId", async ({ params: { orderId } }) => {
    try {
      const order = await payOS.getPaymentLinkInformation(orderId);
      if (!order) {
        return {
          error: -1,
          message: "failed",
          data: null,
        };
      }
      return {
        error: 0,
        message: "ok",
        data: order,
      };
    } catch (error) {
      console.log(error);
      return {
        error: -1,
        message: "failed",
        data: null,
      };
    }
  })
  .put("/:orderId", async ({ params: { orderId }, body }) => {
    try {
      const { cancellationReason } = body as { cancellationReason: string };
      
      console.log(`Cancelling order ${orderId} with reason: ${cancellationReason}`);
      
      const order = await payOS.cancelPaymentLink(orderId, cancellationReason);
      if (!order) {
        return {
          error: -1,
          message: "failed",
          data: null,
        };
      }

      // Get transaction to retrieve subscription ID
      console.log(`[ORDER] Looking up transaction for orderCode: ${orderId}`);
      const transaction = await mongoDBHandler.getTransactionByOrderCode(orderId);
      const subscriptionId = transaction?.subscriptionID || null;
      console.log(`[ORDER] Found subscriptionID: ${subscriptionId}`);

      // Publish payment cancellation event to billing service
      console.log(`[ORDER] Publishing CANCELLATION to billing service...`);
      await rabbitMQService.publishToExchange('billing.events', 'payment.processing', {
        type: 'PAYMENT_CANCELLED',
        orderCode: orderId,
        subscription_id: subscriptionId,
        amount: transaction?.amount || 0,
        description: cancellationReason,
        timestamp: new Date().toISOString(),
        data: {
          cancellationReason: cancellationReason,
          canceledAt: new Date().toISOString(),
          orderData: order
        }
      });
      console.log(`[ORDER] CANCELLATION published to billing service`);

      // Publish to internal payment processing queue
      console.log(`[ORDER] Publishing to internal payment.processing queue...`);
      const paymentMessage: PaymentMessage = {
        type: "PAYMENT_CANCELLED",
        orderCode: orderId,
        amount: transaction?.amount || 0,
        description: cancellationReason,
        timestamp: new Date().toISOString(),
        data: order,
      };

      await rabbitMQService.publishToQueue("payment.processing", paymentMessage);
      console.log(`[ORDER] Internal payment.processing queue published`);

      console.log(`[ORDER] Order ${orderId} cancelled successfully (subscription: ${subscriptionId})`);

      return {
        error: 0,
        message: "ok",
        data: order,
      };
    } catch (error) {
      console.error(`Error cancelling order ${orderId}:`, error);
      return {
        error: -1,
        message: "failed",
        data: null,
      };
    }
  })
  .post("/confirm-webhook", async ({ body }) => {
    const { webhookUrl } = body as { webhookUrl: string };
    try {
      await payOS.confirmWebhook(webhookUrl);
      return {
        error: 0,
        message: "ok",
        data: null,
      };
    } catch (error) {
      console.error(error);
      return {
        error: -1,
        message: "failed",
        data: null,
      };
    }
  })
  .get("/debug/:orderCode", async ({ params: { orderCode } }) => {
    try {
      console.log("Debug: Getting transaction for orderCode:", orderCode);
      
      const transaction = await mongoDBHandler.getTransactionByOrderCode(orderCode);
      
      if (!transaction) {
        return {
          error: -1,
          message: "Transaction not found",
          data: null,
        };
      }
      
      console.log("Debug: Found transaction:", JSON.stringify(transaction, null, 2));
      
      return {
        error: 0,
        message: "Success",
        data: transaction,
      };
    } catch (error) {
      console.error("Error getting transaction for debug:", error);
      return {
        error: -1,
        message: "Failed to get transaction",
        data: null,
      };
    }
  });
