import { rabbitMQService } from '../utils/rabbitmq';
import payOS from '../utils/payos';

export interface PaymentMessage {
  type: 'PAYMENT_CREATED' | 'PAYMENT_SUCCESS' | 'PAYMENT_FAILED' | 'PAYMENT_CANCELLED';
  orderCode: string;
  amount: number;
  description: string;
  timestamp: string;
  data?: any;
}

export interface OrderMessage {
  type: 'ORDER_CREATED' | 'ORDER_UPDATED' | 'ORDER_CANCELLED';
  orderCode: string;
  status: string;
  timestamp: string;
  data?: any;
}

export class PaymentMessageHandler {
  static async handlePaymentProcessing(message: PaymentMessage): Promise<void> {
    console.log('Processing payment message:', message);

    try {
      switch (message.type) {
        case 'PAYMENT_SUCCESS':
          // Handle successful payment
          await PaymentMessageHandler.handlePaymentSuccess(message);
          break;
        case 'PAYMENT_FAILED':
          // Handle failed payment
          await PaymentMessageHandler.handlePaymentFailed(message);
          break;
        case 'PAYMENT_CANCELLED':
          // Handle cancelled payment
          await PaymentMessageHandler.handlePaymentCancelled(message);
          break;
        default:
          console.warn('Unknown payment message type:', message.type);
      }
    } catch (error) {
      console.error('Error handling payment message:', error);
      throw error;
    }
  }

  static async handlePaymentSuccess(message: PaymentMessage): Promise<void> {
    // Send notification
    await rabbitMQService.publishToQueue('payment.notifications', {
      type: 'PAYMENT_SUCCESS_NOTIFICATION',
      orderCode: message.orderCode,
      amount: message.amount,
      timestamp: new Date().toISOString()
    });

    // Update order status
    await rabbitMQService.publishToQueue('order.updates', {
      type: 'ORDER_PAYMENT_COMPLETED',
      orderCode: message.orderCode,
      status: 'PAID',
      timestamp: new Date().toISOString()
    });
  }

  static async handlePaymentFailed(message: PaymentMessage): Promise<void> {
    // Send notification
    await rabbitMQService.publishToQueue('payment.notifications', {
      type: 'PAYMENT_FAILED_NOTIFICATION',
      orderCode: message.orderCode,
      amount: message.amount,
      timestamp: new Date().toISOString()
    });

    // Update order status
    await rabbitMQService.publishToQueue('order.updates', {
      type: 'ORDER_PAYMENT_FAILED',
      orderCode: message.orderCode,
      status: 'PAYMENT_FAILED',
      timestamp: new Date().toISOString()
    });
  }

  static async handlePaymentCancelled(message: PaymentMessage): Promise<void> {
    // Send notification
    await rabbitMQService.publishToQueue('payment.notifications', {
      type: 'PAYMENT_CANCELLED_NOTIFICATION',
      orderCode: message.orderCode,
      timestamp: new Date().toISOString()
    });
  }

  static async handleNotifications(message: any): Promise<void> {
    console.log('Sending notification:', message);
    // Implement your notification logic here (email, SMS, push notifications, etc.)
  }

  static async handleOrderUpdates(message: OrderMessage): Promise<void> {
    console.log('Processing order update:', message);
    // Implement order status update logic here
    // This could involve updating a database, calling another service, etc.
  }
}
