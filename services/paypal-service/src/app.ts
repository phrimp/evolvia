import 'dotenv/config';
import { Elysia } from 'elysia';
import { paypalRoutes } from './routes/paypal.routes';
import { errorHandler, corsMiddleware, requestLogger, paypalValidation } from './middlewares';
import { serviceMonitor } from './utils/monitor';

export const app = new Elysia()
  // Apply middlewares
  .use(corsMiddleware)
  .use(requestLogger)
  .use(errorHandler)
  .use(paypalValidation)
  
  // Monitoring middleware
  .onBeforeHandle(() => {
    const startTime = Date.now();
    return { startTime };
  })
  .onAfterResponse(({ set, request, store }: any) => {
    const responseTime = Date.now() - (store?.startTime || 0);
    const success = set.status < 400;
    const endpoint = new URL(request.url).pathname;
    
    serviceMonitor.recordRequest(success, responseTime, endpoint);
  })
  
  // Health check endpoint
  .get('/', () => {
    const health = serviceMonitor.getHealthStatus();
    return {
      service: 'PayPal Service',
      status: 'running',
      version: '1.0.0',
      timestamp: new Date().toISOString(),
      environment: process.env.PAYPAL_MODE || 'sandbox',
      health: health.status,
      uptime: serviceMonitor.getMetrics().uptime
    };
  })
  
  // Metrics endpoint
  .get('/metrics', () => {
    return serviceMonitor.getMetrics();
  })
  
  // Health endpoint with detailed status
  .get('/health', () => {
    return serviceMonitor.getHealthStatus();
  })
  
  // API Documentation endpoint
  .get('/api', () => ({
    service: 'PayPal Service API',
    version: '1.0.0',
    endpoints: {
      'POST /api/paypal/create-order': 'Create a new PayPal order',
      'POST /api/paypal/capture-order/:orderId': 'Capture an approved PayPal order',
      'GET /api/paypal/order/:orderId': 'Get PayPal order details',
      'GET /api/paypal/success': 'PayPal success callback',
      'GET /api/paypal/cancel': 'PayPal cancel callback',
      'POST /api/paypal/webhook': 'PayPal webhook handler',
      'GET /api/paypal/health': 'PayPal service health check',
      'GET /metrics': 'Service metrics',
      'GET /health': 'Detailed health status'
    },
    documentation: 'Visit /swagger for detailed API documentation'
  }))
  
  // Register PayPal routes
  .use(paypalRoutes);