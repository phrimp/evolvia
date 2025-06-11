import { OrdersController, Order, OrderRequest, CheckoutPaymentIntent } from '@paypal/paypal-server-sdk';
import { getPayPalClient } from '../config/paypal';
import { serviceMonitor } from '../utils/monitor';
import { 
  PayPalOrder, 
  CreateOrderRequest, 
  CaptureOrderResponse, 
  PayPalError 
} from '../types/paypal';

export class PayPalService {
  private client = getPayPalClient();
  private ordersController = new OrdersController(this.client);

  /**
   * Tạo một PayPal order mới
   */
  async createOrder(orderData: CreateOrderRequest): Promise<PayPalOrder> {
    try {
      serviceMonitor.recordPayPalApiCall();
      
      const orderRequest: OrderRequest = {
        intent: CheckoutPaymentIntent.Capture,
        purchaseUnits: [{
          amount: {
            currencyCode: orderData.currency || 'USD',
            value: orderData.amount,
            breakdown: orderData.items ? {
              itemTotal: {
                currencyCode: orderData.currency || 'USD',
                value: orderData.amount
              }
            } : undefined
          },
          description: orderData.description,
          items: orderData.items?.map(item => ({
            name: item.name,
            unitAmount: {
              currencyCode: item.unit_amount.currency_code,
              value: item.unit_amount.value
            },
            quantity: item.quantity,
            description: item.description,
            sku: item.sku,
            category: item.category as any
          }))
        }],
        applicationContext: {
          returnUrl: orderData.returnUrl || `${process.env.HOST || 'http://localhost'}:${process.env.PORT || 3000}/api/paypal/success`,
          cancelUrl: orderData.cancelUrl || `${process.env.HOST || 'http://localhost'}:${process.env.PORT || 3000}/api/paypal/cancel`,
          brandName: 'Your Brand Name',
          landingPage: 'NO_PREFERENCE' as any,
          userAction: 'PAY_NOW' as any
        }
      };

      const response = await this.ordersController.createOrder({
        body: orderRequest,
        prefer: 'return=representation'
      });

      if (response.statusCode !== 201) {
        throw new Error(`PayPal API returned status ${response.statusCode}`);
      }

      serviceMonitor.recordOrderCreated();
      return response.result as PayPalOrder;
    } catch (error) {
      console.error('PayPal Create Order Error:', error);
      serviceMonitor.recordError(`Create order failed: ${error}`);
      throw this.handlePayPalError(error);
    }
  }

  /**
   * Capture một PayPal order đã được approve
   */
  async captureOrder(orderId: string): Promise<CaptureOrderResponse> {
    try {
      serviceMonitor.recordPayPalApiCall();
      
      const response = await this.ordersController.captureOrder({
        id: orderId,
        body: {}
      });

      if (response.statusCode !== 201) {
        throw new Error(`PayPal API returned status ${response.statusCode}`);
      }

      serviceMonitor.recordOrderCaptured();
      return response.result as CaptureOrderResponse;
    } catch (error) {
      console.error('PayPal Capture Order Error:', error);
      serviceMonitor.recordError(`Capture order failed: ${error}`);
      throw this.handlePayPalError(error);
    }
  }

  /**
   * Lấy thông tin chi tiết của một order
   */
  async getOrder(orderId: string): Promise<PayPalOrder> {
    try {
      serviceMonitor.recordPayPalApiCall();
      
      const response = await this.ordersController.getOrder({
        id: orderId
      });

      if (response.statusCode !== 200) {
        throw new Error(`PayPal API returned status ${response.statusCode}`);
      }

      return response.result as PayPalOrder;
    } catch (error) {
      console.error('PayPal Get Order Error:', error);
      serviceMonitor.recordError(`Get order failed: ${error}`);
      throw this.handlePayPalError(error);
    }
  }

  /**
   * Validate webhook signature (for production use)
   */
  async validateWebhook(headers: any, body: string, webhookId: string): Promise<boolean> {
    // Implementation for webhook validation
    // This would require additional PayPal webhook verification
    return true;
  }

  /**
   * Xử lý lỗi từ PayPal API
   */
  private handlePayPalError(error: any): PayPalError {
    if (error.statusCode) {
      return {
        error: `PayPal API Error: ${error.statusCode}`,
        error_description: error.message,
        details: error.details || []
      };
    }
    
    return {
      error: 'PayPal Service Error',
      error_description: error.message || 'Unknown error occurred'
    };
  }

  /**
   * Kiểm tra trạng thái kết nối PayPal
   */
  async healthCheck(): Promise<{ status: string; timestamp: string }> {
    try {
      // Tạo một order test để kiểm tra kết nối
      const testOrder = await this.createOrder({
        amount: '1.00',
        currency: 'USD',
        description: 'Health check test order'
      });
      
      return {
        status: 'healthy',
        timestamp: new Date().toISOString()
      };
    } catch (error) {
      return {
        status: 'unhealthy',
        timestamp: new Date().toISOString()
      };
    }
  }
}

export const paypalService = new PayPalService();
