import { Elysia, t } from 'elysia';
import { isValidAmount, isValidCurrency, isValidEmail, isValidPayPalOrderId } from '../utils/helpers';

export const paypalValidationSchemas = {
  createOrder: {
    body: t.Object({
      amount: t.String({ 
        description: 'Amount to charge (e.g., "10.00")',
        examples: ['10.00', '25.50', '100.00']
      }),
      currency: t.Optional(t.String({ 
        description: 'Currency code (ISO 4217)',
        examples: ['USD', 'EUR', 'GBP'],
        maxLength: 3,
        minLength: 3
      })),
      description: t.Optional(t.String({ 
        description: 'Order description',
        maxLength: 127
      })),
      items: t.Optional(t.Array(t.Object({
        name: t.String({ description: 'Item name' }),
        unit_amount: t.Object({
          currency_code: t.String(),
          value: t.String()
        }),
        quantity: t.String({ description: 'Item quantity' }),
        description: t.Optional(t.String()),
        sku: t.Optional(t.String()),
        category: t.Optional(t.Union([
          t.Literal('DIGITAL_GOODS'), 
          t.Literal('PHYSICAL_GOODS')
        ]))
      }))),
      returnUrl: t.Optional(t.String({ 
        description: 'Success return URL',
        format: 'uri'
      })),
      cancelUrl: t.Optional(t.String({ 
        description: 'Cancel return URL',
        format: 'uri'
      }))
    })
  },

  orderId: {
    params: t.Object({
      orderId: t.String({ 
        description: 'PayPal Order ID',
        minLength: 10,
        examples: ['5O190127TN364715T']
      })
    })
  },

  webhookQuery: {
    query: t.Object({
      token: t.Optional(t.String()),
      PayerID: t.Optional(t.String())
    })
  }
};

export const paypalValidationPlugin = new Elysia({ name: 'paypal-validation' })
  .derive(({ body, params }: any) => {
    // Custom validation for create order
    if (body?.amount) {
      if (!isValidAmount(body.amount)) {
        throw new Error('Invalid amount. Must be a positive number with up to 2 decimal places and max 10000');
      }
    }

    if (body?.currency) {
      if (!isValidCurrency(body.currency)) {
        throw new Error('Invalid currency code. Must be a valid ISO 4217 currency code');
      }
    }

    // Validate items if provided
    if (body?.items && Array.isArray(body.items)) {
      body.items.forEach((item: any, index: number) => {
        if (!isValidAmount(item.unit_amount?.value)) {
          throw new Error(`Invalid amount for item ${index + 1}`);
        }
        
        const quantity = parseInt(item.quantity);
        if (isNaN(quantity) || quantity <= 0 || quantity > 999) {
          throw new Error(`Invalid quantity for item ${index + 1}. Must be between 1 and 999`);
        }
      });
    }

    // Validate PayPal Order ID
    if (params?.orderId) {
      if (!isValidPayPalOrderId(params.orderId)) {
        throw new Error('Invalid PayPal Order ID format');
      }
    }

    return {};
  });

export const rateLimitPlugin = new Elysia({ name: 'rate-limit' })
  .state('requestCounts', new Map<string, { count: number; resetTime: number }>())
  .derive(({ request, store }: any) => {
    const ip = request.headers.get('x-forwarded-for') || 
               request.headers.get('x-real-ip') || 
               'unknown';
    
    const now = Date.now();
    const windowMs = 60 * 1000; // 1 minute
    const maxRequests = 100; // 100 requests per minute
    
    const requestData = store.requestCounts.get(ip) || { count: 0, resetTime: now + windowMs };
    
    if (now > requestData.resetTime) {
      // Reset the window
      requestData.count = 1;
      requestData.resetTime = now + windowMs;
    } else {
      requestData.count++;
    }
    
    store.requestCounts.set(ip, requestData);
    
    if (requestData.count > maxRequests) {
      throw new Error(`Rate limit exceeded. Maximum ${maxRequests} requests per minute allowed.`);
    }
    
    return {};
  });
