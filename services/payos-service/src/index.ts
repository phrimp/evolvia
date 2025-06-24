import { Elysia } from "elysia";
import { paymentController } from "./controllers/payment.controller";
import { orderController } from "./controllers/order.controller";
import { rabbitMQService } from "./utils/rabbitmq";
import { PaymentMessageHandler } from "./handlers/payment.handler";
import { mongoDBHandler } from "./handlers/mongodb.handler";
import cors from "@elysiajs/cors";

// Debug environment variables
console.log("🔍 Environment check:");
console.log("PAYOS_CLIENT_ID:", process.env.PAYOS_CLIENT_ID ? "✅ Set" : "❌ Missing");
console.log("PAYOS_API_KEY:", process.env.PAYOS_API_KEY ? "✅ Set" : "❌ Missing");
console.log("PAYOS_CHECKSUM_KEY:", process.env.PAYOS_CHECKSUM_KEY ? "✅ Set" : "❌ Missing");
console.log("MONGODB_URL:", process.env.MONGO_URI ? "✅ Set" : "❌ Missing (using default: mongodb://localhost:27017)");

// Initialize MongoDB
async function initializeMongoDB() {
  try {
    await mongoDBHandler.connect();
    console.log("MongoDB connected successfully");
  } catch (error) {
    console.error("Failed to connect to MongoDB:", error);
  }
}

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
  .use(cors({
    origin: "*", // Cho phép tất cả domains
    methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"],
    allowedHeaders: ["Content-Type", "Authorization", "X-Requested-With"],
    credentials: false // Set false khi dùng wildcard
  }))
  .get("/", () => "Hello Elysia")
  .group("/protected", (app) => 
    app
      .use(paymentController)
      .use(orderController)
  )
  .listen(process.env.PAYOS_SERVICE_PORT ?? 3000);

// Initialize MongoDB and RabbitMQ
initializeMongoDB();
initializeRabbitMQ();

// Graceful shutdown
process.on("SIGINT", async () => {
  console.log("Shutting down...");
  await rabbitMQService.close();
  await mongoDBHandler.disconnect();
  process.exit(0);
});

console.log(
  `🦊 Elysia/PayOS is running at ${app.server?.hostname}:${app.server?.port}`
);
