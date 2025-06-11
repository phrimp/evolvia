import { Elysia, t } from 'elysia';
import { paypalController } from '../controllers/paypal.controller';
import { paypalValidationSchemas, paypalValidationPlugin, rateLimitPlugin } from '../middlewares/validation';

export const paypalRoutes = new Elysia({ prefix: '/api/paypal' })
  .use(rateLimitPlugin)
  .use(paypalValidationPlugin)
  
  // Tạo PayPal order mới
  .post('/create-order', async ({ body }) => {
    return await paypalController.createOrder(body);
  }, {
    body: paypalValidationSchemas.createOrder.body,
    detail: {
      summary: 'Create PayPal Order',
      description: 'Creates a new PayPal order for payment processing',
      tags: ['PayPal']
    }
  })
  // Capture PayPal order
  .post('/capture-order/:orderId', async ({ params: { orderId } }) => {
    return await paypalController.captureOrder(orderId);
  }, {
    params: paypalValidationSchemas.orderId.params,
    detail: {
      summary: 'Capture PayPal Order',
      description: 'Captures an approved PayPal order',
      tags: ['PayPal']
    }
  })

  // Lấy thông tin order
  .get('/order/:orderId', async ({ params: { orderId } }) => {
    return await paypalController.getOrder(orderId);
  }, {
    params: paypalValidationSchemas.orderId.params,
    detail: {
      summary: 'Get PayPal Order',
      description: 'Retrieves PayPal order details',
      tags: ['PayPal']
    }
  })

  // Success callback từ PayPal
  .get('/success', async ({ query }) => {
    const result = await paypalController.handleSuccess(query);
    
    if (result.success) {
      return new Response(`
        <!DOCTYPE html>
        <html>
        <head>
          <title>Payment Successful</title>
          <style>
            body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
            .success { color: #28a745; }
            .container { max-width: 600px; margin: 0 auto; }
          </style>
        </head>
        <body>
          <div class="container">
            <h1 class="success">✅ Payment Successful!</h1>
            <p>Your payment has been processed successfully.</p>            <p><strong>Order ID:</strong> ${result.data?.orderId || 'N/A'}</p>
            <p><strong>Status:</strong> ${result.data?.status || 'N/A'}</p>
            <button onclick="window.close()">Close Window</button>
          </div>
        </body>
        </html>
      `, {
        headers: { 'Content-Type': 'text/html' }
      });
    } else {
      return new Response(`
        <!DOCTYPE html>
        <html>
        <head>
          <title>Payment Failed</title>
          <style>
            body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
            .error { color: #dc3545; }
            .container { max-width: 600px; margin: 0 auto; }
          </style>
        </head>
        <body>
          <div class="container">
            <h1 class="error">❌ Payment Failed</h1>
            <p>${result.message || 'Payment processing failed'}</p>
            <button onclick="window.close()">Close Window</button>
          </div>
        </body>
        </html>
      `, {
        headers: { 'Content-Type': 'text/html' }
      });
    }  }, {
    query: paypalValidationSchemas.webhookQuery.query,
    detail: {
      summary: 'PayPal Success Callback',
      description: 'Handles successful PayPal payment callback',
      tags: ['PayPal']
    }
  })

  // Cancel callback từ PayPal
  .get('/cancel', async ({ query }) => {
    const result = await paypalController.handleCancel(query);
    
    return new Response(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>Payment Cancelled</title>
        <style>
          body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
          .warning { color: #ffc107; }
          .container { max-width: 600px; margin: 0 auto; }
        </style>
      </head>
      <body>
        <div class="container">
          <h1 class="warning">⚠️ Payment Cancelled</h1>
          <p>Your payment was cancelled. No charges have been made.</p>
          <p><strong>Order ID:</strong> ${result.orderId || 'N/A'}</p>
          <button onclick="window.close()">Close Window</button>
        </div>
      </body>
      </html>
    `, {
      headers: { 'Content-Type': 'text/html' }
    });
  }, {
    query: t.Object({
      token: t.Optional(t.String())
    }),
    detail: {
      summary: 'PayPal Cancel Callback',
      description: 'Handles cancelled PayPal payment callback',
      tags: ['PayPal']
    }
  })

  // Webhook endpoint cho PayPal events
  .post('/webhook', async ({ body, headers }) => {
    return await paypalController.handleWebhook(headers, body);
  }, {
    body: t.Any(),
    detail: {
      summary: 'PayPal Webhook',
      description: 'Handles PayPal webhook events',
      tags: ['PayPal']
    }
  })

  // Health check endpoint
  .get('/health', async () => {
    return await paypalController.healthCheck();
  }, {
    detail: {
      summary: 'PayPal Service Health Check',
      description: 'Checks the health status of PayPal service integration',
      tags: ['Health']
    }
  });
