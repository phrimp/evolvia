import { Elysia } from "elysia";
import { paymentController } from "./controllers/payment.controller";
import { orderController } from "./controllers/order.controller";
import { webhookReceiverController } from "./controllers/webhook.controller"; // Add this import
import { rabbitMQService, EventTracker } from "./utils/rabbitmq";
import { PaymentMessageHandler } from "./handlers/payment.handler";
import { mongoDBHandler } from "./handlers/mongodb.handler";
import cors from "@elysiajs/cors";

// Debug controller inline
const debugController = new Elysia({ prefix: "/debug" })
  .get("/events", () => {
    return {
      success: true,
      totalEvents: EventTracker.getEvents().length,
      events: EventTracker.getEvents()
    };
  })
  .get("/events/order/:orderCode", ({ params: { orderCode } }) => {
    const events = EventTracker.getEvents().filter(e => e.orderCode === orderCode);
    return {
      success: true,
      orderCode,
      totalEvents: events.length,
      events
    };
  })
  .get("/events/subscription/:subscriptionId", ({ params: { subscriptionId } }) => {
    const events = EventTracker.getEvents().filter(e => e.subscriptionId === subscriptionId);
    return {
      success: true,
      subscriptionId,
      totalEvents: events.length,
      events
    };
  })
  .delete("/events", () => {
    EventTracker.getEvents().length = 0; // Clear events
    return {
      success: true,
      message: "All events cleared"
    };
  })
  .get("/events/stats", () => {
    const events = EventTracker.getEvents();
    const stats = {
      total: events.length,
      successful: events.filter(e => e.success).length,
      failed: events.filter(e => !e.success).length,
      byType: {
        exchange: events.filter(e => e.type === 'EXCHANGE').length,
        queue: events.filter(e => e.type === 'QUEUE').length
      },
      byDestination: events.reduce((acc, event) => {
        acc[event.destination] = (acc[event.destination] || 0) + 1;
        return acc;
      }, {} as Record<string, number>),
      byMessageType: events.reduce((acc, event) => {
        acc[event.messageType] = (acc[event.messageType] || 0) + 1;
        return acc;
      }, {} as Record<string, number>)
    };
    return {
      success: true,
      stats
    };
  });

// Debug environment variables
console.log("Environment check:");
console.log("PAYOS_CLIENT_ID:", process.env.PAYOS_CLIENT_ID ? "Set" : "Missing");
console.log("PAYOS_API_KEY:", process.env.PAYOS_API_KEY ? "Set" : "Missing");
console.log("PAYOS_CHECKSUM_KEY:", process.env.PAYOS_CHECKSUM_KEY ? "Set" : "Missing");
console.log("MONGO_URI:", process.env.MONGO_URI ? "Set" : "Missing");
console.log("PORT:", process.env.PORT || process.env.PAYOS_SERVICE_PORT || 9250);
console.log("RABBITMQ_URI:", process.env.RABBITMQ_URI ? "Set" : "Missing");
console.log("NODE_ENV:", process.env.NODE_ENV || "development");

// Initialize MongoDB
async function initializeMongoDB() {
  try {
    console.log("Connecting to MongoDB...");
    await mongoDBHandler.connect();
    console.log("MongoDB connected successfully");
  } catch (error) {
    console.error("Failed to connect to MongoDB:", error);
    console.log("Service will continue without MongoDB connection");
  }
}

// Initialize RabbitMQ and setup consumers
async function initializeRabbitMQ() {
  try {
    console.log("Connecting to RabbitMQ...");
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
    console.log("Service will continue without RabbitMQ connection");
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
    console.error(`Elysia Error [${code}]:`, error);
    return { error: `Server error: ${String(error)}`, code };
  })
  .get("/", () => "Hello Elysia PayOS Service with Debug Logging!")
  .group("/public/payos", (app) => 
    app
      .get("/ping", () => ({ message: "pong!" }))
      .use(paymentController) // PayOS webhooks should be public
      .use(webhookReceiverController) // Test webhook receiver
  )
  .group("/protected/payos", (app) => 
    app
      .get("/ping", () => ({ message: "pong!" }))
      .use(orderController) // Order creation requires authentication
  )
  .group("/debug/payos", (app) =>
    app.use(debugController) // Debug endpoints
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
console.log(`Elysia/PayOS is running at 0.0.0.0:${port}`);
console.log(`Debug endpoints available at:`);
console.log(`   GET  /debug/payos/events - View all events`);
console.log(`   GET  /debug/payos/events/order/{orderCode} - Events for specific order`);
console.log(`   GET  /debug/payos/events/subscription/{subscriptionId} - Events for specific subscription`);
console.log(`   GET  /debug/payos/events/stats - Event statistics`);
console.log(`   DELETE /debug/payos/events - Clear all events`);
