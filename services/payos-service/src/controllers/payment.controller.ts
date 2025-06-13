import { Elysia } from "elysia";
import payOS from "../utils/payos";
import { rabbitMQService } from "../utils/rabbitmq";
import { PaymentMessage } from "../handlers/payment.handler";

export const paymentController = new Elysia({ prefix: "/payment" })
  .post("/payos", async ({ body, headers }) => {
    console.log("payment handler");
    
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

      // Publish payment event to RabbitMQ for async processing
      const paymentMessage: PaymentMessage = {
        type: webhookData.code === '00' ? 'PAYMENT_SUCCESS' : 'PAYMENT_FAILED',
        orderCode: webhookData.orderCode.toString(),
        amount: webhookData.amount,
        description: webhookData.description,
        timestamp: new Date().toISOString(),
        data: webhookData
      };

      await rabbitMQService.publishToQueue('payment.processing', paymentMessage);

      console.log("Payment message published to queue:", paymentMessage);

      return {
        error: 0,
        message: "Ok",
        data: webhookData
      };
    } catch (error) {
      console.error("Webhook verification failed:", error);
      console.error("Error details:", error instanceof Error ? error.message : String(error));
      console.error("Request body:", body);
      
      // Publish failed payment message
      try {
        await rabbitMQService.publishToQueue('payment.failed', {
          type: 'WEBHOOK_VERIFICATION_FAILED',
          error: error instanceof Error ? error.message : String(error),
          body: body,
          timestamp: new Date().toISOString()
        });
      } catch (queueError) {
        console.error("Failed to publish error to queue:", queueError);
      }
      
      return {
        error: -1,
        message: "Webhook verification failed",
        data: null
      };
    }
  });