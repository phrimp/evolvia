import * as amqp from 'amqplib';

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

        // Payment processing queue
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
    }

    async publishToQueue(queue: string, message: any): Promise<void> {
        if (!this.channel) {
            await this.connect();
        }

        if (!this.channel) {
            throw new Error('Failed to establish channel connection');
        }

        const messageBuffer = Buffer.from(JSON.stringify(message));

        return new Promise((resolve, reject) => {
            const result = this.channel!.sendToQueue(queue, messageBuffer, { persistent: true });
            if (result) {
                resolve();
            } else {
                reject(new Error('Failed to send message to queue'));
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