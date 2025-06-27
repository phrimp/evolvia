import { Elysia } from "elysia";
import payOS from "../utils/payos";
import { rabbitMQService } from "../utils/rabbitmq";
import { PaymentMessage } from "../handlers/payment.handler";
import { mongoDBHandler } from "../handlers/mongodb.handler";

export const orderController = new Elysia({ prefix: "/order" })  .post("/create", async ({ body }) => {
    console.log("📦 Order creation request received:", body);
    
    const { userId, description, returnUrl, cancelUrl, amount } = body as {
      userId: string;
      description: string;
      returnUrl: string;
      cancelUrl: string;
      amount: number;
    };

    const orderData = {
      orderCode: Number(String(new Date().getTime()).slice(-6)),
      amount,
      description,
      cancelUrl,
      returnUrl,
      expiredAt: Math.floor((Date.now() + 30 * 1000) / 1000), // 30 seconds from now (Unix timestamp)
    };

    console.log("📦 Order data prepared:", orderData);

    try {
      console.log("💳 Creating PayOS payment link...");
      const paymentLinkRes = await payOS.createPaymentLink(orderData);
      console.log("✅ PayOS payment link created:", paymentLinkRes);

      // Auto-cancel after 30 seconds as backup
      setTimeout(async () => {
        try {
          const linkInfo = await payOS.getPaymentLinkInformation(orderData.orderCode);
          if (linkInfo && linkInfo.status === 'PENDING') {
            console.log(`⏰ Auto-cancelling expired order: ${orderData.orderCode}`);
            await payOS.cancelPaymentLink(orderData.orderCode, "Payment link expired after 30 seconds");
            
            // Publish timeout event
            await rabbitMQService.publishToQueue("payment.processing", {
              type: "PAYMENT_TIMEOUT",
              orderCode: orderData.orderCode.toString(),
              timestamp: new Date().toISOString(),
              data: { reason: "30 second timeout" }
            });
          }
        } catch (error) {
          console.error(`❌ Failed to auto-cancel order ${orderData.orderCode}:`, error);
        }
      }, 30 * 1000); // 30 seconds

      // Save transaction to MongoDB (only userId, orderCode, checkoutUrl, subscriptionId)
      await mongoDBHandler.createTransaction({
        userId,
        orderCode: orderData.orderCode.toString(),
        checkoutUrl: paymentLinkRes.checkoutUrl,
      });

      // Publish order creation event
      await rabbitMQService.publishToQueue("order.updates", {
        type: "ORDER_CREATED",
        orderCode: orderData.orderCode.toString(),
        timestamp: new Date().toISOString(),
        data: {
          userId,
          amount: orderData.amount,
          description: orderData.description,
          checkoutUrl: paymentLinkRes.checkoutUrl,
        },
      });

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
        },
      };} catch (error) {
      console.error("❌ PayOS error details:", error);
      console.error("❌ Error message:", error instanceof Error ? error.message : String(error));
      console.error("❌ Error stack:", error instanceof Error ? error.stack : 'No stack trace');

      // Publish order creation failure
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
      };    }
  })
  .get("/user/:userId", async ({ params: { userId } }) => {
    try {
      console.log("👤 Getting orders for user:", userId);
      
      const transactions = await mongoDBHandler.getTransactionsByUserId(userId);
      
      return {
        error: 0,
        message: "Success",
        data: transactions,
      };
    } catch (error) {
      console.error("❌ Error getting user orders:", error);
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
      const order = await payOS.cancelPaymentLink(orderId, cancellationReason);
      if (!order) {
        return {
          error: -1,
          message: "failed",
          data: null,
        };
      }

      // Publish payment cancellation event
      const paymentMessage: PaymentMessage = {
        type: "PAYMENT_CANCELLED",
        orderCode: orderId,
        amount: 0, // We don't have amount info here
        description: cancellationReason,
        timestamp: new Date().toISOString(),
        data: order,
      };

      await rabbitMQService.publishToQueue("payment.processing", paymentMessage);

      return {
        error: 0,
        message: "ok",
        data: order,
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
  });