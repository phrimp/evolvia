import { Elysia } from 'elysia';

export const errorHandler = new Elysia()
  .onError(({ code, error, set }) => {
    console.error(`Error [${code}]:`, error);

    // Helper function to safely get error message
    const getErrorMessage = (err: any): string => {
      if (typeof err === 'string') return err;
      if (err && typeof err.message === 'string') return err.message;
      if (err && typeof err.toString === 'function') return err.toString();
      return 'Unknown error occurred';
    };

    switch (code) {
      case 'VALIDATION':
        set.status = 400;
        return {
          success: false,
          error: 'Validation Error',
          details: getErrorMessage(error),
          timestamp: new Date().toISOString()
        };

      case 'NOT_FOUND':
        set.status = 404;
        return {
          success: false,
          error: 'Not Found',
          details: 'The requested resource was not found',
          timestamp: new Date().toISOString()
        };

      case 'INTERNAL_SERVER_ERROR':
      default:
        set.status = 500;
        return {
          success: false,
          error: 'Internal Server Error',
          details: getErrorMessage(error),
          timestamp: new Date().toISOString()
        };
    }
  });

export const corsMiddleware = new Elysia()
  .onRequest(({ set, request }) => {
    // Set CORS headers
    set.headers['Access-Control-Allow-Origin'] = '*';
    set.headers['Access-Control-Allow-Methods'] = 'GET, POST, PUT, DELETE, OPTIONS';
    set.headers['Access-Control-Allow-Headers'] = 'Content-Type, Authorization';
    
    // Handle preflight requests
    if (request.method === 'OPTIONS') {
      set.status = 200;
      return new Response(null);
    }
  });

export const requestLogger = new Elysia()
  .onRequest(({ request }) => {
    console.log(`[${new Date().toISOString()}] ${request.method} ${new URL(request.url).pathname}`);
  })
  .onAfterResponse(({ request, set }) => {
    console.log(`[${new Date().toISOString()}] ${request.method} ${new URL(request.url).pathname} - ${set.status || 200}`);
  });

export const paypalValidation = new Elysia()
  .onBeforeHandle(({ request }) => {
    const path = new URL(request.url).pathname;
    
    // Skip validation for health check and static endpoints
    if (path.includes('/health') || path.includes('/success') || path.includes('/cancel')) {
      return;
    }

    // Check if PayPal credentials are configured
    if (!process.env.PAYPAL_CLIENT_ID || !process.env.PAYPAL_CLIENT_SECRET) {
      throw new Error('PayPal credentials not configured. Please set PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET in environment variables.');
    }
  });
