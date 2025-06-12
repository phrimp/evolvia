import { Elysia, t } from "elysia";
import payOS from "../utils/payos";

export const orderController = new Elysia({ prefix: "/order" })
  .post("/create", async ({ body }) => {
    const { description, returnUrl, cancelUrl, amount } = body as {
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
    };

    try {
      const paymentLinkRes = await payOS.createPaymentLink(orderData);

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
      };
    } catch (error) {
      console.log(error);
      return {
        error: -1,
        message: "fail",
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
      const order = await payOS.cancelPaymentLink(orderId, cancellationReason);
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
      console.error(error);
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