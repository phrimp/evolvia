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
console.log("MONGO_URI:", process.env.MONGO_URI ? "✅ Set" : "❌ Missing");
console.log("PORT:", process.env.PORT || process.env.PAYOS_SERVICE_PORT || 9250);
console.log("RABBITMQ_URI:", process.env.RABBITMQ_URI ? "✅ Set" : "❌ Missing");
console.log("NODE_ENV:", process.env.NODE_ENV || "development");

// Initialize MongoDB
async function initializeMongoDB() {
  try {
    console.log("🔄 Connecting to MongoDB...");
    await mongoDBHandler.connect();
    console.log("✅ MongoDB connected successfully");
  } catch (error) {
    console.error("❌ Failed to connect to MongoDB:", error);
    // Don't exit process, let service continue
    console.log("⚠️  Service will continue without MongoDB connection");
  }
}

// Initialize RabbitMQ and setup consumers
async function initializeRabbitMQ() {
  try {
    console.log("🔄 Connecting to RabbitMQ...");
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

    console.log("✅ RabbitMQ consumers initialized");
  } catch (error) {
    console.error("❌ Failed to initialize RabbitMQ:", error);
    console.log("⚠️  Service will continue without RabbitMQ connection");
  }
}

const app = new Elysia()
  .use(cors({
    origin: true,
    methods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
    allowedHeaders: ['Content-Type', 'Authorization'],
    credentials: true
  }))
  .onError(({ error, code }) => {
    console.error(`❌ Elysia Error [${code}]:`, error);
    return { error: `Server error: ${String(error)}`, code };
  })
  .get("/", () => "Hello Elysia")
  .group("/public/payos", (app) => 
    app
      .get("/test", () => ({ message: "Public PayOS route works!" }))
      .use(paymentController) // PayOS webhooks should be public
  )
  .group("/protected/payos", (app) => 
    app
      .get("/test", () => ({ message: "Protected PayOS route works!" }))
      .use(orderController) // Order creation requires authentication
  )
  .listen({
    port: Number(process.env.PORT || process.env.PAYOS_SERVICE_PORT || 9250),
    hostname: "0.0.0.0"
  });

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

const port = Number(process.env.PORT || process.env.PAYOS_SERVICE_PORT || 9250);
console.log(`🦊 Elysia/PayOS is running at 0.0.0.0:${port}`);
