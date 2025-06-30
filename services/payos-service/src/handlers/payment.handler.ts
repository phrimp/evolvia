import { rabbitMQService } from '../utils/rabbitmq';
import { mongoDBHandler } from './mongodb.handler';

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
    console.log(`[HANDLER] Processing payment success for order ${message.orderCode}`);

    try {
      // Get transaction to retrieve subscription ID
      console.log(`[HANDLER] Looking up transaction for orderCode: ${message.orderCode}`);
      const transaction = await mongoDBHandler.getTransactionByOrderCode(message.orderCode);
      const subscriptionId = transaction?.subscriptionID || null;
      console.log(`[HANDLER] Found subscriptionID: ${subscriptionId}`);

      // Publish to billing service (CRITICAL)
      console.log(`[HANDLER] Publishing SUCCESS to billing service...`);
      await rabbitMQService.publishToExchange('billing.events', 'payment.processing', {
        type: 'PAYMENT_SUCCESS',
        orderCode: message.orderCode,
        subscription_id: subscriptionId,
        amount: message.amount,
        description: message.description,
        timestamp: new Date().toISOString(),
        data: {
          paymentId: message.paymentDetails?.id,
          status: message.paymentDetails?.status,
          accountNumber: message.paymentDetails?.accountNumber,
          accountName: message.paymentDetails?.accountName,
          amountPaid: message.paymentDetails?.amountPaid || message.amount,
          transactionDetails: message.paymentDetails?.transactions
        }
      });
      console.log(`[HANDLER] SUCCESS published to billing service`);

      console.log(`[HANDLER] Publishing internal notifications...`);
      // Enhanced notification with all payment data
      await rabbitMQService.publishToQueue('payment.notifications', {
        type: 'PAYMENT_SUCCESS_NOTIFICATION',
        orderCode: message.orderCode,
        amount: message.amount,
        amountPaid: message.paymentDetails?.amountPaid || message.amount,
        paymentId: message.paymentDetails?.id,
        accountName: message.paymentDetails?.accountName,
        subscriptionId: subscriptionId,
        timestamp: new Date().toISOString(),
        fullPaymentData: message.paymentDetails
      });

      // Enhanced order update with complete payment info
      await rabbitMQService.publishToQueue('order.updates', {
        type: 'ORDER_PAYMENT_COMPLETED',
        orderCode: message.orderCode,
        status: 'PAID',
        subscriptionId: subscriptionId,
        paymentDetails: message.paymentDetails,
        timestamp: new Date().toISOString()
      });

      // Publish to public events with all data
      await rabbitMQService.publishToQueue('public.payment.events', {
        eventType: 'PAYMENT_COMPLETED',
        orderCode: message.orderCode,
        subscriptionId: subscriptionId,
        paymentData: message.paymentDetails,
        timestamp: new Date().toISOString()
      });

      console.log(`[HANDLER] Payment success events published for order ${message.orderCode} (subscription: ${subscriptionId})`);

    } catch (error) {
      console.error(`[HANDLER] Error handling payment success for ${message.orderCode}:`, error);
      throw error;
    }
  }

  static async handlePaymentFailed(message: PaymentMessage): Promise<void> {
    console.log(`[HANDLER] Processing payment failure for order ${message.orderCode}`);

    try {
      // Get transaction to retrieve subscription ID
      console.log(`[HANDLER] Looking up transaction for orderCode: ${message.orderCode}`);
      const transaction = await mongoDBHandler.getTransactionByOrderCode(message.orderCode);
      const subscriptionId = transaction?.subscriptionID || null;
      console.log(`[HANDLER] Found subscriptionID: ${subscriptionId}`);

      // Publish to billing service (CRITICAL)
      console.log(`[HANDLER] Publishing FAILURE to billing service...`);
      await rabbitMQService.publishToExchange('billing.events', 'payment.processing', {
        type: 'PAYMENT_FAILED',
        orderCode: message.orderCode,
        subscription_id: subscriptionId,
        amount: message.amount,
        description: message.description,
        timestamp: new Date().toISOString(),
        data: {
          paymentId: message.paymentDetails?.id,
          status: message.paymentDetails?.status,
          failureReason: message.description
        }
      });
      console.log(`[HANDLER] FAILURE published to billing service`);

      console.log(`[HANDLER] Publishing internal notifications...`);
      // Send notification
      await rabbitMQService.publishToQueue('payment.notifications', {
        type: 'PAYMENT_FAILED_NOTIFICATION',
        orderCode: message.orderCode,
        amount: message.amount,
        subscriptionId: subscriptionId,
        timestamp: new Date().toISOString()
      });

      // Update order status
      await rabbitMQService.publishToQueue('order.updates', {
        type: 'ORDER_PAYMENT_FAILED',
        orderCode: message.orderCode,
        status: 'PAYMENT_FAILED',
        subscriptionId: subscriptionId,
        timestamp: new Date().toISOString()
      });

      console.log(`[HANDLER] Payment failure events published for order ${message.orderCode} (subscription: ${subscriptionId})`);

    } catch (error) {
      console.error(`[HANDLER] Error handling payment failure for ${message.orderCode}:`, error);
      throw error;
    }
  }

  static async handlePaymentCancelled(message: PaymentMessage): Promise<void> {
    console.log(`[HANDLER] Processing payment cancellation for order ${message.orderCode}`);

    try {
      // Get transaction to retrieve subscription ID
      console.log(`[HANDLER] Looking up transaction for orderCode: ${message.orderCode}`);
      const transaction = await mongoDBHandler.getTransactionByOrderCode(message.orderCode);
      const subscriptionId = transaction?.subscriptionID || null;
      console.log(`[HANDLER] Found subscriptionID: ${subscriptionId}`);

      // Publish to billing service (CRITICAL)
      console.log(`[HANDLER] Publishing CANCELLATION to billing service...`);
      await rabbitMQService.publishToExchange('billing.events', 'payment.processing', {
        type: 'PAYMENT_CANCELLED',
        orderCode: message.orderCode,
        subscription_id: subscriptionId,
        amount: message.amount,
        description: message.description,
        timestamp: new Date().toISOString(),
        data: {
          paymentId: message.paymentDetails?.id,
          status: message.paymentDetails?.status,
          canceledAt: message.paymentDetails?.canceledAt,
          cancellationReason: message.paymentDetails?.cancellationReason
        }
      });
      console.log(`[HANDLER] CANCELLATION published to billing service`);

      console.log(`[HANDLER] Publishing internal notifications...`);
      // Enhanced cancellation notification
      await rabbitMQService.publishToQueue('payment.notifications', {
        type: 'PAYMENT_CANCELLED_NOTIFICATION',
        orderCode: message.orderCode,
        amount: message.amount,
        amountRemaining: message.paymentDetails?.amountRemaining,
        canceledAt: message.paymentDetails?.canceledAt,
        cancellationReason: message.paymentDetails?.cancellationReason,
        subscriptionId: subscriptionId,
        timestamp: new Date().toISOString(),
        fullPaymentData: message.paymentDetails
      });

      // Update order with cancellation details
      await rabbitMQService.publishToQueue('order.updates', {
        type: 'ORDER_PAYMENT_CANCELLED',
        orderCode: message.orderCode,
        status: 'CANCELLED',
        subscriptionId: subscriptionId,
        cancellationDetails: {
          canceledAt: message.paymentDetails?.canceledAt,
          reason: message.paymentDetails?.cancellationReason
        },
        paymentDetails: message.paymentDetails,
        timestamp: new Date().toISOString()
      });

      console.log(`[HANDLER] Payment cancellation events published for order ${message.orderCode} (subscription: ${subscriptionId})`);

    } catch (error) {
      console.error(`[HANDLER] Error handling payment cancellation for ${message.orderCode}:`, error);
      throw error;
    }
  }

  static async handlePaymentTimeout(message: PaymentMessage): Promise<void> {
    console.log(`[HANDLER] Processing payment timeout for order ${message.orderCode}`);

    try {
      // Get transaction to retrieve subscription ID
      console.log(`[HANDLER] Looking up transaction for orderCode: ${message.orderCode}`);
      const transaction = await mongoDBHandler.getTransactionByOrderCode(message.orderCode);
      const subscriptionId = transaction?.subscriptionID || null;
      console.log(`[HANDLER] Found subscriptionID: ${subscriptionId}`);

      // Publish to billing service (CRITICAL)
      console.log(`[HANDLER] Publishing TIMEOUT to billing service...`);
      await rabbitMQService.publishToExchange('billing.events', 'payment.processing', {
        type: 'PAYMENT_TIMEOUT',
        orderCode: message.orderCode,
        subscription_id: subscriptionId,
        amount: message.amount,
        description: message.description || 'Payment timeout',
        timestamp: new Date().toISOString(),
        data: {
          paymentId: message.paymentDetails?.id,
          status: message.paymentDetails?.status,
          timeoutReason: 'Payment expired'
        }
      });
      console.log(`[HANDLER] TIMEOUT published to billing service`);

      console.log(`[HANDLER] Payment timeout events published for order ${message.orderCode} (subscription: ${subscriptionId})`);

    } catch (error) {
      console.error(`[HANDLER] Error handling payment timeout for ${message.orderCode}:`, error);
      throw error;
    }
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
