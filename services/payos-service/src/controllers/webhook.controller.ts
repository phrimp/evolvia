import { Elysia } from "elysia";
import { rabbitMQService } from "../utils/rabbitmq";
import { mongoDBHandler } from "../handlers/mongodb.handler";

export const webhookReceiverController = new Elysia({ prefix: "/webhook" })
  .post("/payment", async ({ body, headers }) => {
    console.log("Test webhook received");
    console.log("Headers:", headers);
    console.log("Body:", JSON.stringify(body, null, 2));
    
    try {
      const webhookData = body as {
        code: string;
        desc: string;
        success: boolean;
        data: any;
        signature: string;
      };

      // Extract order information
      const orderCodeRaw = webhookData.data?.orderCode || "TEST_ORDER_" + Date.now();
      const orderCode = orderCodeRaw.toString();
      const amount = webhookData.data?.amount || 2000;
      const description = webhookData.desc || "Test payment";

      // Determine payment status based on code
      const isSuccess = webhookData.code === "00" || webhookData.success === true;
      const paymentType = isSuccess ? 'PAYMENT_SUCCESS' : 'PAYMENT_FAILED';

      console.log(`Processing test webhook for order: ${orderCode} - Type: ${paymentType}`);
      console.log(`DEBUG: orderCode type: ${typeof orderCode}, value: "${orderCode}"`);

      // Get transaction from MongoDB to retrieve subscription ID
      const transaction = await mongoDBHandler.getTransactionByOrderCode(orderCode);
      const subscriptionId = transaction?.subscriptionID || null;

      console.log("Transaction lookup result:");
      console.log("  - orderCode:", orderCode);
      console.log("  - found transaction:", !!transaction);
      console.log("  - subscriptionID:", subscriptionId);
      console.log("  - payment result:", isSuccess ? "SUCCESS" : "FAILED");

      // Create billing service event
      const billingServiceEvent = {
        type: paymentType,
        orderCode: orderCode,
        subscription_id: subscriptionId,
        amount: amount,
        description: description,
        timestamp: new Date().toISOString(),
        data: {
          paymentId: webhookData.data?.id || 'TEST_PAYMENT_' + Date.now(),
          status: isSuccess ? 'PAID' : 'FAILED',
          accountNumber: webhookData.data?.accountNumber || '12345678',
          accountName: webhookData.data?.accountName || 'TEST ACCOUNT',
          amountPaid: isSuccess ? amount : 0,
          failureReason: isSuccess ? undefined : webhookData.desc,
          testWebhook: true
        }
      };

      console.log(`[BILLING] Publishing test ${paymentType} to billing service...`);
      console.log("[BILLING] Event details:", {
        type: billingServiceEvent.type,
        orderCode: billingServiceEvent.orderCode,
        subscription_id: billingServiceEvent.subscription_id,
        exchange: 'billing.events',
        routingKey: 'payment.processing'
      });

      // Publish to billing service exchange (CRITICAL FOR BILLING SERVICE)
      await rabbitMQService.publishToExchange(
        'billing.events',
        'payment.processing',
        billingServiceEvent
      );
      console.log(`[BILLING] Test ${paymentType} event published to billing service`);

      // Also publish to internal queues for completeness
      await rabbitMQService.publishToQueue('payment.processing', {
        type: paymentType,
        orderCode: orderCode,
        amount: amount,
        description: description,
        timestamp: new Date().toISOString(),
        data: webhookData.data,
        testWebhook: true
      });

      console.log(`[WEBHOOK] Test webhook processed successfully - ${paymentType}`);

      return {
        success: true
      };

    } catch (error) {
      console.error("Error processing test webhook:", error);
      
      // Still return success to acknowledge webhook receipt
      return {
        success: true
      };
    }
  });
