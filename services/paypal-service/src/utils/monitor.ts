import { EventEmitter } from 'events';

export interface ServiceMetrics {
  totalRequests: number;
  successfulRequests: number;
  failedRequests: number;
  averageResponseTime: number;
  uptime: number;
  memoryUsage: NodeJS.MemoryUsage;
  paypalApiCalls: number;
  webhooksReceived: number;
  ordersCreated: number;
  ordersCaptured: number;
  errors: Array<{
    timestamp: Date;
    error: string;
    endpoint?: string;
  }>;
}

class ServiceMonitor extends EventEmitter {
  private static instance: ServiceMonitor;
  private metrics: ServiceMetrics;
  private startTime: Date;
  private responseTimes: number[] = [];
  private maxResponseTimes = 100; // Keep last 100 response times

  static getInstance(): ServiceMonitor {
    if (!ServiceMonitor.instance) {
      ServiceMonitor.instance = new ServiceMonitor();
    }
    return ServiceMonitor.instance;
  }

  constructor() {
    super();
    this.startTime = new Date();
    this.metrics = {
      totalRequests: 0,
      successfulRequests: 0,
      failedRequests: 0,
      averageResponseTime: 0,
      uptime: 0,
      memoryUsage: process.memoryUsage(),
      paypalApiCalls: 0,
      webhooksReceived: 0,
      ordersCreated: 0,
      ordersCaptured: 0,
      errors: []
    };

    // Update metrics periodically
    setInterval(() => {
      this.updateMetrics();
    }, 30000); // Every 30 seconds
  }

  /**
   * Record a request
   */
  recordRequest(success: boolean, responseTime?: number, endpoint?: string): void {
    this.metrics.totalRequests++;
    
    if (success) {
      this.metrics.successfulRequests++;
    } else {
      this.metrics.failedRequests++;
    }

    if (responseTime) {
      this.responseTimes.push(responseTime);
      if (this.responseTimes.length > this.maxResponseTimes) {
        this.responseTimes.shift();
      }
      this.calculateAverageResponseTime();
    }

    this.emit('request', { success, responseTime, endpoint });
  }

  /**
   * Record PayPal API call
   */
  recordPayPalApiCall(): void {
    this.metrics.paypalApiCalls++;
    this.emit('paypal-api-call');
  }

  /**
   * Record webhook received
   */
  recordWebhook(): void {
    this.metrics.webhooksReceived++;
    this.emit('webhook-received');
  }

  /**
   * Record order created
   */
  recordOrderCreated(): void {
    this.metrics.ordersCreated++;
    this.emit('order-created');
  }

  /**
   * Record order captured
   */
  recordOrderCaptured(): void {
    this.metrics.ordersCaptured++;
    this.emit('order-captured');
  }

  /**
   * Record error
   */
  recordError(error: string, endpoint?: string): void {
    const errorRecord = {
      timestamp: new Date(),
      error,
      endpoint
    };

    this.metrics.errors.push(errorRecord);
    
    // Keep only last 50 errors
    if (this.metrics.errors.length > 50) {
      this.metrics.errors.shift();
    }

    this.emit('error', errorRecord);
  }

  /**
   * Get current metrics
   */
  getMetrics(): ServiceMetrics {
    this.updateMetrics();
    return { ...this.metrics };
  }

  /**
   * Get health status
   */
  getHealthStatus(): {
    status: 'healthy' | 'warning' | 'critical';
    details: any;
  } {
    const metrics = this.getMetrics();
    const successRate = metrics.totalRequests > 0 
      ? (metrics.successfulRequests / metrics.totalRequests) * 100 
      : 100;

    const recentErrors = metrics.errors.filter(
      e => Date.now() - e.timestamp.getTime() < 5 * 60 * 1000 // Last 5 minutes
    );

    let status: 'healthy' | 'warning' | 'critical' = 'healthy';
    
    if (successRate < 50 || recentErrors.length > 10) {
      status = 'critical';
    } else if (successRate < 80 || recentErrors.length > 5) {
      status = 'warning';
    }

    return {
      status,
      details: {
        successRate: successRate.toFixed(2),
        recentErrors: recentErrors.length,
        averageResponseTime: metrics.averageResponseTime,
        uptime: metrics.uptime,
        memoryUsage: {
          used: (metrics.memoryUsage.heapUsed / 1024 / 1024).toFixed(2) + ' MB',
          total: (metrics.memoryUsage.heapTotal / 1024 / 1024).toFixed(2) + ' MB'
        }
      }
    };
  }

  /**
   * Reset metrics
   */
  resetMetrics(): void {
    this.metrics = {
      totalRequests: 0,
      successfulRequests: 0,
      failedRequests: 0,
      averageResponseTime: 0,
      uptime: 0,
      memoryUsage: process.memoryUsage(),
      paypalApiCalls: 0,
      webhooksReceived: 0,
      ordersCreated: 0,
      ordersCaptured: 0,
      errors: []
    };
    this.responseTimes = [];
    this.startTime = new Date();
    this.emit('metrics-reset');
  }

  /**
   * Update calculated metrics
   */
  private updateMetrics(): void {
    this.metrics.uptime = Date.now() - this.startTime.getTime();
    this.metrics.memoryUsage = process.memoryUsage();
    this.calculateAverageResponseTime();
  }

  /**
   * Calculate average response time
   */
  private calculateAverageResponseTime(): void {
    if (this.responseTimes.length > 0) {
      const sum = this.responseTimes.reduce((a, b) => a + b, 0);
      this.metrics.averageResponseTime = Math.round(sum / this.responseTimes.length);
    }
  }

  /**
   * Export metrics for external monitoring systems
   */
  exportMetrics(): string {
    const metrics = this.getMetrics();
    const timestamp = new Date().toISOString();
    
    return JSON.stringify({
      timestamp,
      service: 'paypal-service',
      metrics,
      health: this.getHealthStatus()
    }, null, 2);
  }
}

export const serviceMonitor = ServiceMonitor.getInstance();
