import { Elysia } from "elysia";
import payOS from "../utils/payos";
import { rabbitMQService } from "../utils/rabbitmq";
import { PaymentMessage } from "../handlers/payment.handler";
import { mongoDBHandler } from "../handlers/mongodb.handler";
import { MongoClient, ObjectId } from 'mongodb';

// Helper function to create subscription directly in billing MongoDB
async function createSubscription(userId: string): Promise<string | null> {
  let client: MongoClient | null = null;
  try {
    console.log(`📋 Creating subscription for user ${userId} in billing MongoDB...`);
    
    // Connect to billing service MongoDB
    const billingMongoUrl = process.env.BILLING_MONGO_URI || process.env.MONGO_URI || 'mongodb://root:example@mongodb:27017';
    client = new MongoClient(billingMongoUrl);
    await client.connect();
    
    const billingDb = client.db('billing_management_service');
    const subscriptionsCollection = billingDb.collection('subscriptions');
    
    // Generate subscription ID
    const subscriptionId = `sub_${new ObjectId().toString()}`;
    
    // Insert subscription document
    const subscriptionDoc = {
      _id: new ObjectId(),
      subscriptionId: subscriptionId,
      userId: userId,
      createdAt: new Date(),
    };
    
    const result = await subscriptionsCollection.insertOne(subscriptionDoc);
    
    if (result.insertedId) {
      console.log(`✅ Subscription created: ${subscriptionId}`);
      return subscriptionId;
    } else {
      console.error("❌ Failed to insert subscription");
      return null;
    }
    
  } catch (error) {
    console.error("❌ Error creating subscription:", error);
    return null;
  } finally {
    if (client) {
      await client.close();
    }
  }
}

// Timeout manager to track pending orders
class OrderTimeoutManager {
  private timeouts: Map<string, NodeJS.Timeout> = new Map();

  scheduleTimeout(orderCode: string, timeoutMs: number) {
    console.log(`⏰ Scheduling timeout for order ${orderCode} in ${timeoutMs}ms`);
    
    const timeoutId = setTimeout(async () => {
      try {
        console.log(`🚨 Order ${orderCode} timed out, attempting to cancel...`);
        
        // Call PayOS directly instead of going through protected API
        const order = await payOS.cancelPaymentLink(orderCode, "Timeout - Payment link expired after 30 seconds");
        
        if (order) {
          console.log(`✅ Order ${orderCode} cancelled due to timeout`);
        } else {
          console.error(`❌ Failed to cancel order ${orderCode}: No order returned`);
        }

        // Remove from tracking
        this.timeouts.delete(orderCode);

        // Publish timeout event
        await rabbitMQService.publishToQueue("payment.processing", {
          type: "PAYMENT_TIMEOUT",
          orderCode: orderCode,
          timestamp: new Date().toISOString(),
          data: { reason: "30 second timeout", auto_cancelled: true }
        });

      } catch (error) {
        console.error(`❌ Failed to auto-cancel order ${orderCode}:`, error);
        this.timeouts.delete(orderCode);
      }
    }, timeoutMs);

    this.timeouts.set(orderCode, timeoutId);
  }

  cancelTimeout(orderCode: string) {
    const timeoutId = this.timeouts.get(orderCode);
    if (timeoutId) {
      clearTimeout(timeoutId);
      this.timeouts.delete(orderCode);
      console.log(`⏰ Cancelled timeout for order ${orderCode}`);
    }
  }

  clearAllTimeouts() {
    this.timeouts.forEach((timeoutId, orderCode) => {
      clearTimeout(timeoutId);
      console.log(`⏰ Cleared timeout for order ${orderCode}`);
    });
    this.timeouts.clear();
  }
}

const orderTimeoutManager = new OrderTimeoutManager();

// Export for use in payment handlers
export { orderTimeoutManager };

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
      // Step 1: Create subscription in billing_management_service FIRST
      console.log("� Creating subscription first...");
      const subscriptionId = await createSubscription(userId);
      
      if (!subscriptionId) {
        console.error("❌ Failed to create subscription, aborting order creation");
        return {
          error: -1,
          message: "Failed to create subscription",
          data: null,
        };
      }
      
      console.log(`✅ Subscription created: ${subscriptionId}`);

      // Step 2: Create PayOS payment link
      console.log("�💳 Creating PayOS payment link...");
      const paymentLinkRes = await payOS.createPaymentLink(orderData);
      console.log("✅ PayOS payment link created:", paymentLinkRes);

      // Schedule auto-cancel after 30 seconds using Worker Thread approach
      const orderCode = orderData.orderCode.toString();
      orderTimeoutManager.scheduleTimeout(orderCode, 30000); // 30 seconds

      // Step 3: Save transaction to MongoDB with subscriptionId
      console.log("💾 Saving transaction to MongoDB...");
      await mongoDBHandler.createTransaction({
        userId,
        orderCode: orderData.orderCode.toString(),
        checkoutUrl: paymentLinkRes.checkoutUrl,
        subscriptionId: subscriptionId, // Use the subscription ID from step 1
      });
      console.log("✅ Transaction saved to MongoDB");

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
          subscriptionId: subscriptionId, // Include subscription ID in event
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
      
      console.log(`🚫 Cancelling order ${orderId} with reason: ${cancellationReason}`);
      
      const order = await payOS.cancelPaymentLink(orderId, cancellationReason);
      if (!order) {
        return {
          error: -1,
          message: "failed",
          data: null,
        };
      }

      // Cancel the timeout since order is being manually cancelled
      orderTimeoutManager.cancelTimeout(orderId);

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

      console.log(`✅ Order ${orderId} cancelled successfully`);

      return {
        error: 0,
        message: "ok",
        data: order,
      };
    } catch (error) {
      console.error(`❌ Error cancelling order ${orderId}:`, error);
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