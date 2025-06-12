import { Elysia } from "elysia";
import payOS from "../utils/payos";

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

      // Process the webhook data here
      console.log("Processing webhook data:", webhookData);
      
      // Add your business logic here based on the webhook data
      // For example, update order status, send notifications, etc.

      return {
        error: 0,
        message: "Ok",
        data: webhookData
      };
    } catch (error) {
      console.error("Webhook verification failed:", error);
      console.error("Error details:", error instanceof Error ? error.message : String(error));
      console.error("Request body:", body);
      
      return {
        error: -1,
        message: "Webhook verification failed",
        data: null
      };
    }
  });