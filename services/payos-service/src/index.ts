import { Elysia } from "elysia";
import { paymentController } from "./controllers/payment.controller";
import { orderController } from "./controllers/order.controller";
import { rabbitMQService } from "./utils/rabbitmq";
import { PaymentMessageHandler } from "./handlers/payment.handler";

// Debug environment variables
console.log("🔍 Environment check:");
console.log("PAYOS_CLIENT_ID:", process.env.PAYOS_CLIENT_ID ? "✅ Set" : "❌ Missing");
console.log("PAYOS_API_KEY:", process.env.PAYOS_API_KEY ? "✅ Set" : "❌ Missing");
console.log("PAYOS_CHECKSUM_KEY:", process.env.PAYOS_CHECKSUM_KEY ? "✅ Set" : "❌ Missing");

// Initialize RabbitMQ and setup consumers
async function initializeRabbitMQ() {
  try {
    await rabbitMQService.connect();

    // Setup message consumers
    await rabbitMQService.consumeFromQueue(
      "payment.processing",
      PaymentMessageHandler.handlePaymentProcessing
    );
    await rabbitMQService.consumeFromQueue(
      "payment.notifications",
      PaymentMessageHandler.handleNotifications
    );
    await rabbitMQService.consumeFromQueue(
      "order.updates",
      PaymentMessageHandler.handleOrderUpdates
    );

    console.log("RabbitMQ consumers initialized");
  } catch (error) {
    console.error("Failed to initialize RabbitMQ:", error);
  }
}

const app = new Elysia()
  .get("/", () => "Hello Elysia")
  .group("/protected", (app) => 
    app
      .use(paymentController)
      .use(orderController)
  )
  .listen(process.env.ELYSIA_PORT ?? 3000);

// Initialize RabbitMQ
initializeRabbitMQ();

// Graceful shutdown
process.on("SIGINT", async () => {
  console.log("Shutting down...");
  await rabbitMQService.close();
  process.exit(0);
});

console.log(
  `🦊 Elysia/PayOS is running at ${app.server?.hostname}:${app.server?.port}`
);
