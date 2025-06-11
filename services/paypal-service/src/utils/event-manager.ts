import { EventEmitter } from 'events';

export interface PayPalWebhookEvent {
  id: string;
  event_type: string;
  create_time: string;
  resource_type: string;
  resource: any;
  summary?: string;
  links?: Array<{
    href: string;
    rel: string;
    method: string;
  }>;
}

export type PayPalEventType = 
  | 'PAYMENT.CAPTURE.COMPLETED'
  | 'PAYMENT.CAPTURE.DENIED'
  | 'PAYMENT.CAPTURE.PENDING'
  | 'PAYMENT.CAPTURE.REFUNDED'
  | 'PAYMENT.CAPTURE.REVERSED'
  | 'CHECKOUT.ORDER.APPROVED'
  | 'CHECKOUT.ORDER.COMPLETED'
  | 'CHECKOUT.ORDER.CANCELLED'
  | 'PAYMENT.AUTHORIZATION.CREATED'
  | 'PAYMENT.AUTHORIZATION.VOIDED'
  | 'BILLING.SUBSCRIPTION.CREATED'
  | 'BILLING.SUBSCRIPTION.ACTIVATED'
  | 'BILLING.SUBSCRIPTION.CANCELLED';

class PayPalEventManager extends EventEmitter {
  private static instance: PayPalEventManager;
  private processedEvents = new Set<string>();

  static getInstance(): PayPalEventManager {
    if (!PayPalEventManager.instance) {
      PayPalEventManager.instance = new PayPalEventManager();
    }
    return PayPalEventManager.instance;
  }

  constructor() {
    super();
    this.setupDefaultHandlers();
  }

  /**
   * Process a webhook event
   */
  async processWebhookEvent(event: PayPalWebhookEvent): Promise<void> {
    try {
      // Check for duplicate events
      if (this.processedEvents.has(event.id)) {
        console.log(`Event ${event.id} already processed, skipping`);
        return;
      }

      // Mark as processed
      this.processedEvents.add(event.id);

      // Clean up old processed events (keep last 1000)
      if (this.processedEvents.size > 1000) {
        const toDelete = Array.from(this.processedEvents).slice(0, 100);
        toDelete.forEach(id => this.processedEvents.delete(id));
      }

      console.log(`Processing webhook event: ${event.event_type} (${event.id})`);

      // Emit the specific event type
      this.emit(event.event_type, event);

      // Emit a general webhook event
      this.emit('webhook', event);

      // Log the event for audit purposes
      this.logEvent(event);

    } catch (error) {
      console.error('Error processing webhook event:', error);
      this.emit('error', error, event);
    }
  }

  /**
   * Setup default event handlers
   */
  private setupDefaultHandlers(): void {
    // Payment completed handler
    this.on('PAYMENT.CAPTURE.COMPLETED', (event: PayPalWebhookEvent) => {
      console.log(`✅ Payment completed: ${event.resource.id}`);
      // Add your business logic here
      // Example: Update order status, send confirmation email, etc.
    });

    // Payment denied handler
    this.on('PAYMENT.CAPTURE.DENIED', (event: PayPalWebhookEvent) => {
      console.log(`❌ Payment denied: ${event.resource.id}`);
      // Add your business logic here
      // Example: Update order status, notify customer, etc.
    });

    // Order approved handler
    this.on('CHECKOUT.ORDER.APPROVED', (event: PayPalWebhookEvent) => {
      console.log(`👍 Order approved: ${event.resource.id}`);
      // Add your business logic here
      // Example: Prepare order for fulfillment
    });

    // Order completed handler
    this.on('CHECKOUT.ORDER.COMPLETED', (event: PayPalWebhookEvent) => {
      console.log(`🎉 Order completed: ${event.resource.id}`);
      // Add your business logic here
      // Example: Start fulfillment process
    });

    // Payment refunded handler
    this.on('PAYMENT.CAPTURE.REFUNDED', (event: PayPalWebhookEvent) => {
      console.log(`💰 Payment refunded: ${event.resource.id}`);
      // Add your business logic here
      // Example: Update inventory, send refund notification
    });

    // Error handler
    this.on('error', (error: Error, event?: PayPalWebhookEvent) => {
      console.error('PayPal webhook error:', error);
      if (event) {
        console.error('Failed event:', event.id, event.event_type);
      }
    });
  }

  /**
   * Register a custom event handler
   */
  onPayPalEvent(eventType: PayPalEventType, handler: (event: PayPalWebhookEvent) => void): void {
    this.on(eventType, handler);
  }

  /**
   * Register a handler for all webhook events
   */
  onAnyWebhookEvent(handler: (event: PayPalWebhookEvent) => void): void {
    this.on('webhook', handler);
  }

  /**
   * Log event for audit purposes
   */
  private logEvent(event: PayPalWebhookEvent): void {
    const logData = {
      timestamp: new Date().toISOString(),
      eventId: event.id,
      eventType: event.event_type,
      resourceType: event.resource_type,
      resourceId: event.resource?.id,
      summary: event.summary
    };

    // In a real application, you might want to store this in a database
    console.log('Webhook event logged:', JSON.stringify(logData));
  }
  /**
   * Get statistics about processed events
   */
  getEventStats(): { processedCount: number; eventTypes: string[] } {
    const eventTypes = this.eventNames().map(event => event.toString());
    return {
      processedCount: this.processedEvents.size,
      eventTypes: Array.from(new Set(eventTypes))
    };
  }
}

export const paypalEventManager = PayPalEventManager.getInstance();
