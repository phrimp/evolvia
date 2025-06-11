import { paypalService } from '../services/paypal.service';
import { CreateOrderRequest } from '../types/paypal';

export class PayPalController {
  /**
   * Tạo PayPal order mới
   */
  async createOrder(body: CreateOrderRequest) {
    try {
      const order = await paypalService.createOrder(body);
      
      return {
        success: true,
        data: {
          orderId: order.id,
          status: order.status,
          approvalUrl: order.links.find(link => link.rel === 'approve')?.href,
          order: order
        }
      };
    } catch (error: any) {
      return {
        success: false,
        error: error.error || 'Failed to create PayPal order',
        details: error.error_description || error.message
      };
    }
  }

  /**
   * Capture PayPal order
   */
  async captureOrder(orderId: string) {
    try {
      const result = await paypalService.captureOrder(orderId);
      
      return {
        success: true,
        data: {
          orderId: result.id,
          status: result.status,
          payer: result.payer,
          purchaseUnits: result.purchase_units,
          captureResult: result
        }
      };
    } catch (error: any) {
      return {
        success: false,
        error: error.error || 'Failed to capture PayPal order',
        details: error.error_description || error.message
      };
    }
  }

  /**
   * Lấy thông tin order
   */
  async getOrder(orderId: string) {
    try {
      const order = await paypalService.getOrder(orderId);
      
      return {
        success: true,
        data: order
      };
    } catch (error: any) {
      return {
        success: false,
        error: error.error || 'Failed to get PayPal order',
        details: error.error_description || error.message
      };
    }
  }

  /**
   * Xử lý success callback từ PayPal
   */
  async handleSuccess(query: { token?: string; PayerID?: string }) {
    try {
      if (!query.token) {
        throw new Error('Missing order token');
      }

      const order = await paypalService.getOrder(query.token);
        if (order.status === 'APPROVED') {
        const captureResult = await paypalService.captureOrder(query.token);
        return {
          success: true,
          message: 'Payment completed successfully',
          data: {
            orderId: captureResult.id,
            status: captureResult.status,
            captureResult
          }
        };
      }

      return {
        success: false,
        message: 'Payment not approved',
        data: { order }
      };
    } catch (error: any) {
      return {
        success: false,
        error: 'Payment processing failed',
        details: error.message
      };
    }
  }

  /**
   * Xử lý cancel callback từ PayPal
   */
  async handleCancel(query: { token?: string }) {
    return {
      success: false,
      message: 'Payment was cancelled by user',
      orderId: query.token
    };
  }  /**
   * Webhook handler cho PayPal events
   */
  async handleWebhook(headers: any, body: any) {
    try {
      // Import utilities
      const { webhookVerifier } = await import('../utils/webhook-verifier');
      const { paypalEventManager } = await import('../utils/event-manager');
      const { serviceMonitor } = await import('../utils/monitor');

      // Record webhook received
      serviceMonitor.recordWebhook();

      // Extract verification data
      const verificationData = webhookVerifier.extractWebhookData(headers, body);

      // Validate webhook event structure
      if (!webhookVerifier.validateWebhookEvent(body)) {
        return {
          success: false,
          error: 'Invalid webhook event structure',
          details: 'Event is missing required fields'
        };
      }

      // Verify webhook signature (in production)
      const isVerified = await webhookVerifier.verifyWebhookSignature(verificationData);
      if (!isVerified) {
        console.warn('Webhook signature verification failed');
        return {
          success: false,
          error: 'Webhook verification failed',
          details: 'Invalid signature or transmission data'
        };
      }

      // Process the webhook event
      await paypalEventManager.processWebhookEvent(body);

      return {
        success: true,
        message: 'Webhook processed successfully',
        eventId: body.id,
        eventType: body.event_type
      };

    } catch (error: any) {
      console.error('Webhook processing error:', error);
      const { serviceMonitor } = await import('../utils/monitor');
      serviceMonitor.recordError(`Webhook processing failed: ${error.message}`);
      return {
        success: false,
        error: 'Webhook processing failed',
        details: error.message
      };
    }
  }

  /**
   * Xử lý payment completed event
   */
  private async handlePaymentCompleted(resource: any) {
    console.log('Payment completed:', resource.id);
    // Thêm logic xử lý business khi payment hoàn thành
    // Ví dụ: cập nhật database, gửi email confirmation, etc.
    
    return {
      success: true,
      message: 'Payment completed event processed'
    };
  }

  /**
   * Xử lý payment denied event
   */
  private async handlePaymentDenied(resource: any) {
    console.log('Payment denied:', resource.id);
    // Thêm logic xử lý khi payment bị từ chối
    
    return {
      success: true,
      message: 'Payment denied event processed'
    };
  }

  /**
   * Xử lý payment refunded event
   */
  private async handlePaymentRefunded(resource: any) {
    console.log('Payment refunded:', resource.id);
    // Thêm logic xử lý khi payment bị refund
    
    return {
      success: true,
      message: 'Payment refunded event processed'
    };
  }

  /**
   * Health check
   */
  async healthCheck() {
    try {
      const health = await paypalService.healthCheck();
      return {
        success: true,
        service: 'PayPal Service',
        status: health.status,
        timestamp: health.timestamp
      };
    } catch (error: any) {
      return {
        success: false,
        service: 'PayPal Service',
        status: 'unhealthy',
        error: error.message,
        timestamp: new Date().toISOString()
      };
    }
  }
}

export const paypalController = new PayPalController();
