import { rabbitMQService } from '../utils/rabbitmq';

export interface PaymentMessage {
  type: 'PAYMENT_CREATED' | 'PAYMENT_SUCCESS' | 'PAYMENT_FAILED' | 'PAYMENT_CANCELLED';
  orderCode: string;
  amount: number;
  description: string;
  timestamp: string;
  data?: any;
  // Enhanced payment data
  paymentDetails?: {
    id?: string;
    bin?: string;
    checkoutUrl?: string;
    accountNumber?: string;
    accountName?: string;
    qrCode?: string;
    amountPaid?: number;
    amountRemaining?: number;
    status?: string;
    createdAt?: string;
    canceledAt?: string;
    cancellationReason?: string;
    transactions?: any[];
  };
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
    // Since we only store basic transaction info, we don't update status
    // Just send notifications and events
    console.log(`✅ Payment successful for order ${message.orderCode}`);

    // Enhanced notification with all payment data
    await rabbitMQService.publishToQueue('payment.notifications', {
      type: 'PAYMENT_SUCCESS_NOTIFICATION',
      orderCode: message.orderCode,
      amount: message.amount,
      amountPaid: message.paymentDetails?.amountPaid || message.amount,
      paymentId: message.paymentDetails?.id,
      accountName: message.paymentDetails?.accountName,
      timestamp: new Date().toISOString(),
      fullPaymentData: message.paymentDetails
    });

    // Enhanced order update with complete payment info
    await rabbitMQService.publishToQueue('order.updates', {
      type: 'ORDER_PAYMENT_COMPLETED',
      orderCode: message.orderCode,
      status: 'PAID',
      paymentDetails: message.paymentDetails,
      timestamp: new Date().toISOString()
    });

    // Publish to public events with all data
    await rabbitMQService.publishToQueue('public.payment.events', {
      eventType: 'PAYMENT_COMPLETED',
      orderCode: message.orderCode,
      paymentData: message.paymentDetails,
      timestamp: new Date().toISOString()
    });
  }
  static async handlePaymentFailed(message: PaymentMessage): Promise<void> {
    // Since we only store basic transaction info, we don't update status
    // Just send notifications and events
    console.log(`❌ Payment failed for order ${message.orderCode}`);

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
    // Since we only store basic transaction info, we don't update status
    // Just send notifications and events
    console.log(`🚫 Payment cancelled for order ${message.orderCode}`);

    // Enhanced cancellation notification
    await rabbitMQService.publishToQueue('payment.notifications', {
      type: 'PAYMENT_CANCELLED_NOTIFICATION',
      orderCode: message.orderCode,
      amount: message.amount,
      amountRemaining: message.paymentDetails?.amountRemaining,
      canceledAt: message.paymentDetails?.canceledAt,
      cancellationReason: message.paymentDetails?.cancellationReason,
      timestamp: new Date().toISOString(),
      fullPaymentData: message.paymentDetails
    });

    // Update order with cancellation details
    await rabbitMQService.publishToQueue('order.updates', {
      type: 'ORDER_PAYMENT_CANCELLED',
      orderCode: message.orderCode,
      status: 'CANCELLED',
      cancellationDetails: {
        canceledAt: message.paymentDetails?.canceledAt,
        reason: message.paymentDetails?.cancellationReason
      },
      paymentDetails: message.paymentDetails,
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
