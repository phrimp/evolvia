import { Elysia } from "elysia";
import { paymentController } from "./controllers/payment.controller";
import { orderController } from "./controllers/order.controller";

const app = new Elysia()
  .get("/", () => "Hello Elysia")
  .use(paymentController)
  .use(orderController)
  .listen(process.env.PORT ?? 3000);

console.log(
  `🦊 Elysia/PayOS is running at ${app.server?.hostname}:${app.server?.port}`
);