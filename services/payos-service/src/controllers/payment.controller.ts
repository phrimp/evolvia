import { Elysia } from "elysia";
import payOS from "../utils/payos";
import { rabbitMQService } from "../utils/rabbitmq";
import { PaymentMessage } from "../handlers/payment.handler";
import { mongoDBHandler } from "../handlers/mongodb.handler";

export const paymentController = new Elysia({ prefix: "/payment" })
  .post("/payos", async ({ body, headers }) => {
    console.log("PayOS webhook received");
    
    try {
      // PayOS webhook verification
      const webhookData = payOS.verifyPaymentWebhookData(body as any);

      if (
        ["Ma giao dich thu nghiem", "VQRIO123"].includes(webhookData.description)
      ) {
        return {
          error: 0,
          message: "Ok",
          data: webhookData
        };
      }
      const orderCode = webhookData.orderCode.toString();
      console.log(`DEBUG: Webhook orderCode type: ${typeof orderCode}, value: "${orderCode}"`);
      // Get transaction from MongoDB to retrieve subscription ID
      const transaction = await mongoDBHandler.getTransactionByOrderCode(webhookData.orderCode.toString());
      const subscriptionId = transaction?.subscriptionID || null;

      console.log("Transaction lookup result:");
      console.log("  - orderCode:", webhookData.orderCode);
      console.log("  - found transaction:", !!transaction);
      console.log("  - subscriptionID:", subscriptionId);

      // Enhanced payment message with all data
      const paymentMessage: PaymentMessage = {
        type: webhookData.code === '00' ? 'PAYMENT_SUCCESS' : 'PAYMENT_FAILED',
        orderCode: webhookData.orderCode.toString(),
        amount: webhookData.amount,
        description: webhookData.description,
        timestamp: new Date().toISOString(),
        data: webhookData,
        paymentDetails: {
          id: (webhookData as any).id,
          bin: (webhookData as any).bin,
          checkoutUrl: (webhookData as any).checkoutUrl,
          accountNumber: webhookData.accountNumber,
          accountName: (webhookData as any).accountName,
          qrCode: (webhookData as any).qrCode,
          amountPaid: (webhookData as any).amountPaid,
          amountRemaining: (webhookData as any).amountRemaining,
          status: (webhookData as any).status,
          createdAt: (webhookData as any).createdAt,
          canceledAt: (webhookData as any).canceledAt,
          cancellationReason: (webhookData as any).cancellationReason,
          transactions: (webhookData as any).transactions
        }
      };

      // Billing service event (NEW)
      const billingServiceEvent = {
        type: webhookData.code === '00' ? 'PAYMENT_SUCCESS' : 'PAYMENT_FAILED',
        orderCode: webhookData.orderCode.toString(),
        subscription_id: subscriptionId, // Key field for billing service
        amount: webhookData.amount,
        description: webhookData.description,
        timestamp: new Date().toISOString(),
        data: {
          paymentId: (webhookData as any).id,
          status: (webhookData as any).status,
          accountNumber: webhookData.accountNumber,
          accountName: (webhookData as any).accountName,
          amountPaid: (webhookData as any).amountPaid,
          bin: (webhookData as any).bin,
          transactionDetails: (webhookData as any).transactions
        }
      };

      console.log("[BILLING] Preparing to publish billing service event");
      console.log("[BILLING] Event details:", {
        type: billingServiceEvent.type,
        orderCode: billingServiceEvent.orderCode,
        subscription_id: billingServiceEvent.subscription_id,
        exchange: 'billing.events',
        routingKey: 'payment.processing'
      });

      console.log("[EVENT-FLOW] Starting event publishing sequence...");

      // Publish to billing service exchange (CRITICAL FOR BILLING SERVICE)
      console.log("[BILLING] Publishing to billing service exchange...");
      await rabbitMQService.publishToExchange(
        'billing.events',
        'payment.processing',
        billingServiceEvent
      );
      console.log("[BILLING] Billing service event published successfully");

      console.log("[INTERNAL] Publishing to internal queues...");

      // Publish to internal queues and other services
      await Promise.all([
        // Internal payment processing
        (async () => {
          console.log("[QUEUE] Publishing to payment.processing queue...");
          await rabbitMQService.publishToQueue('payment.processing', paymentMessage);
          console.log("[QUEUE] payment.processing queue published");
        })(),
        
        // Public payment events
        (async () => {
          console.log("[QUEUE] Publishing to public.payment.events queue...");
          await rabbitMQService.publishToQueue('public.payment.events', {
            eventType: 'PAYMENT_WEBHOOK_RECEIVED',
            orderCode: paymentMessage.orderCode,
            status: (webhookData as any).status,
            paymentData: paymentMessage.paymentDetails,
            subscriptionId: subscriptionId,
            timestamp: new Date().toISOString()
          });
          console.log("[QUEUE] public.payment.events queue published");
        })(),
        
        // Analytics events
        (async () => {
          console.log("[QUEUE] Publishing to analytics.events queue...");
          await rabbitMQService.publishToQueue('analytics.events', {
            category: 'payment',
            action: (webhookData as any).status,
            orderCode: paymentMessage.orderCode,
            amount: paymentMessage.amount,
            paymentMethod: (webhookData as any).bin ? 'BANK_TRANSFER' : 'UNKNOWN',
            paymentDetails: paymentMessage.paymentDetails,
            subscriptionId: subscriptionId,
            timestamp: new Date().toISOString()
          });
          console.log("[QUEUE] analytics.events queue published");
        })()
      ]);

      console.log("[EVENT-FLOW] All payment events published successfully!");
      console.log("[SUMMARY] Event publishing summary:");
      console.log("  - billing.events exchange (payment.processing routing key)");
      console.log("  - payment.processing queue");
      console.log("  - public.payment.events queue");
      console.log("  - analytics.events queue");

      return {
        error: 0,
        message: "Ok",
        data: webhookData
      };
    } catch (error) {
      console.error("Webhook verification failed:", error);
      
      // Enhanced error event publishing
      try {
        await Promise.all([
          rabbitMQService.publishToQueue('payment.failed', {
            type: 'WEBHOOK_VERIFICATION_FAILED',
            error: error instanceof Error ? error.message : String(error),
            body: body,
            timestamp: new Date().toISOString(),
            rawData: body
          }),
          // Also publish failure to billing service
          rabbitMQService.publishToExchange('billing.events', 'payment.processing', {
            type: 'PAYMENT_FAILED',
            orderCode: 'UNKNOWN',
            subscription_id: null,
            amount: 0,
            description: 'Webhook verification failed',
            timestamp: new Date().toISOString(),
            data: {
              error: error instanceof Error ? error.message : String(error),
              rawBody: body
            }
          })
        ]);
      } catch (queueError) {
        console.error("Failed to publish error events:", queueError);
      }
      
      return {
        error: -1,
        message: "Webhook verification failed",
        data: null
      };
    }
  })
  
  // Add endpoint to manually publish payment events with full data
  .post("/events/publish-full", async ({ body }) => {
    try {
      const { paymentData, eventType = 'MANUAL_PAYMENT_EVENT' } = body as { 
        paymentData: any, 
        eventType?: string 
      };
      
      const fullEvent = {
        eventType,
        orderCode: paymentData.orderCode,
        timestamp: new Date().toISOString(),
        source: 'payos-service',
        paymentDetails: {
          id: paymentData.id,
          bin: paymentData.bin,
          checkoutUrl: paymentData.checkoutUrl,
          accountNumber: paymentData.accountNumber,
          accountName: paymentData.accountName,
          amount: paymentData.amount,
          amountPaid: paymentData.amountPaid,
          amountRemaining: paymentData.amountRemaining,
          qrCode: paymentData.qrCode,
          status: paymentData.status,
          createdAt: paymentData.createdAt,
          canceledAt: paymentData.canceledAt,
          cancellationReason: paymentData.cancellationReason,
          transactions: paymentData.transactions || []
        }
      };

      await rabbitMQService.publishToQueue('public.payment.events', fullEvent);

      return {
        success: true,
        message: "Full payment event published successfully",
        eventId: `${eventType}_${Date.now()}`
      };
    } catch (error) {
      return {
        success: false,
        message: "Failed to publish full payment event",
        error: error instanceof Error ? error.message : String(error)
      };
    }
  });
