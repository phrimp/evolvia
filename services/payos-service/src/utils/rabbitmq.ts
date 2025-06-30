import * as amqp from 'amqplib';

// Event tracking for debugging
interface EventLog {
  timestamp: string;
  type: 'EXCHANGE' | 'QUEUE';
  destination: string;
  routingKey?: string;
  messageType: string;
  orderCode?: string;
  subscriptionId?: string;
  success: boolean;
  error?: string;
}

class EventTracker {
  private static events: EventLog[] = [];
  private static maxEvents = 100;

  static logEvent(event: EventLog) {
    this.events.unshift(event);
    if (this.events.length > this.maxEvents) {
      this.events.pop();
    }
    
    const status = event.success ? 'SUCCESS' : 'FAILED';
    const typeLabel = event.type === 'EXCHANGE' ? '[EXCHANGE]' : '[QUEUE]';
    console.log(`${status} ${typeLabel} ${event.messageType} -> ${event.destination}${event.routingKey ? ` (${event.routingKey})` : ''}`);
    if (event.orderCode) console.log(`   Order: ${event.orderCode}`);
    if (event.subscriptionId) console.log(`   Subscription: ${event.subscriptionId}`);
    if (event.error) console.log(`   Error: ${event.error}`);
  }

  static getEvents(): EventLog[] {
    return this.events;
  }
}

class RabbitMQService {
    private connection: any = null;
    private channel: any = null;
    private readonly url: string;

    constructor() {
        this.url = process.env.RABBITMQ_URI || 'amqp://localhost:5672';
    }
    
    async connect(): Promise<void> {
        try {
            this.connection = await amqp.connect(this.url) as any;
            this.channel = await this.connection.createChannel();

            // Setup exchanges and queues
            await this.setupQueues();

            console.log('Connected to RabbitMQ');
        } catch (error) {
            console.error('Failed to connect to RabbitMQ:', error);
            throw error;
        }
    }

    private async setupQueues(): Promise<void> {
        if (!this.channel) throw new Error('Channel not initialized');

        // Declare the topic exchange for billing service communication
        await this.channel.assertExchange('billing.events', 'topic', {
            durable: true
        });

        // Internal payment processing queue
        await this.channel.assertQueue('payment.processing', {
            durable: true,
            arguments: {
                'x-message-ttl': 300000, // 5 minutes TTL
            }
        });

        // Payment notifications queue
        await this.channel.assertQueue('payment.notifications', {
            durable: true
        });

        // Dead letter queue for failed payments
        await this.channel.assertQueue('payment.failed', {
            durable: true
        });

        // Order updates queue
        await this.channel.assertQueue('order.updates', {
            durable: true
        });

        console.log('RabbitMQ exchanges and queues setup complete');
    }

    async publishToQueue(queue: string, message: any): Promise<void> {
        if (!this.channel) {
            await this.connect();
        }

        if (!this.channel) {
            throw new Error('Failed to establish channel connection');
        }

        const messageBuffer = Buffer.from(JSON.stringify(message));

        console.log(`[QUEUE] Publishing to queue: ${queue}`);
        console.log(`[QUEUE] Message type: ${message.type || message.eventType || 'unknown'}`);
        console.log(`[QUEUE] Message preview:`, {
            type: message.type,
            orderCode: message.orderCode,
            subscription_id: message.subscription_id,
            amount: message.amount
        });

        return new Promise((resolve, reject) => {
            const result = this.channel!.sendToQueue(queue, messageBuffer, { persistent: true });
            
            // Log event
            EventTracker.logEvent({
                timestamp: new Date().toISOString(),
                type: 'QUEUE',
                destination: queue,
                messageType: message.type || message.eventType || 'unknown',
                orderCode: message.orderCode,
                subscriptionId: message.subscription_id || message.subscriptionId,
                success: result,
                error: result ? undefined : 'Failed to send to queue'
            });

            if (result) {
                console.log(`[QUEUE] Successfully published to queue: ${queue}`);
                resolve();
            } else {
                console.error(`[QUEUE] Failed to publish to queue: ${queue}`);
                reject(new Error('Failed to send message to queue'));
            }
        });
    }

    async publishToExchange(exchange: string, routingKey: string, message: any): Promise<void> {
        if (!this.channel) {
            await this.connect();
        }

        if (!this.channel) {
            throw new Error('Failed to establish channel connection');
        }

        const messageBuffer = Buffer.from(JSON.stringify(message));
        
        console.log(`[EXCHANGE] Publishing to exchange: ${exchange}`);
        console.log(`[EXCHANGE] Routing key: ${routingKey}`);
        console.log(`[EXCHANGE] Message type: ${message.type || 'unknown'}`);
        console.log(`[EXCHANGE] Message details:`, {
            type: message.type,
            orderCode: message.orderCode,
            subscription_id: message.subscription_id,
            amount: message.amount,
            timestamp: message.timestamp
        });
        console.log(`[EXCHANGE] Full message:`, JSON.stringify(message, null, 2));
        
        return new Promise((resolve, reject) => {
            const result = this.channel!.publish(exchange, routingKey, messageBuffer, { 
                persistent: true,
                timestamp: Date.now()
            });

            // Log event
            EventTracker.logEvent({
                timestamp: new Date().toISOString(),
                type: 'EXCHANGE',
                destination: exchange,
                routingKey: routingKey,
                messageType: message.type || 'unknown',
                orderCode: message.orderCode,
                subscriptionId: message.subscription_id || message.subscriptionId,
                success: result,
                error: result ? undefined : 'Failed to publish to exchange'
            });

            if (result) {
                console.log(`[EXCHANGE] Successfully published to exchange: ${exchange} with routing key: ${routingKey}`);
                resolve();
            } else {
                console.error(`[EXCHANGE] Failed to publish to exchange: ${exchange} with routing key: ${routingKey}`);
                reject(new Error(`Failed to publish message to exchange: ${exchange}`));
            }
        });
    }

    async consumeFromQueue(queue: string, callback: (message: any) => Promise<void>): Promise<void> {
        if (!this.channel) {
            await this.connect();
        }

        if (!this.channel) {
            throw new Error('Failed to establish channel connection');
        }

        await this.channel.consume(queue, async (msg: any) => {
            if (msg) {
                try {
                    const content = JSON.parse(msg.content.toString());
                    await callback(content);
                    this.channel!.ack(msg);
                } catch (error) {
                    console.error(`Error processing message from ${queue}:`, error);
                    this.channel!.nack(msg, false, false); // Send to dead letter queue
                }
            }
        });
    }

    async close(): Promise<void> {
        try {
            if (this.channel) {
                await this.channel.close();
                this.channel = null;
            }
            if (this.connection) {
                await this.connection.close();
                this.connection = null;
            }
        } catch (error) {
            console.error('Error closing RabbitMQ connection:', error);
            // Force cleanup even if close fails
            this.channel = null;
            this.connection = null;
        }
    }
}

export const rabbitMQService = new RabbitMQService();
export { EventTracker };
