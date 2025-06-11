import crypto from 'crypto';

export interface WebhookVerificationData {
  authAlgorithm: string;
  certId: string;
  signature: string;
  transmissionId: string;
  transmissionTime: string;
  webhookId: string;
  webhookEvent: any;
}

export class PayPalWebhookVerifier {
  private static instance: PayPalWebhookVerifier;
  private certCache = new Map<string, string>();

  static getInstance(): PayPalWebhookVerifier {
    if (!PayPalWebhookVerifier.instance) {
      PayPalWebhookVerifier.instance = new PayPalWebhookVerifier();
    }
    return PayPalWebhookVerifier.instance;
  }

  /**
   * Verify PayPal webhook signature
   * Note: This is a simplified implementation. For production use,
   * you should implement full webhook verification as per PayPal documentation
   */
  async verifyWebhookSignature(data: WebhookVerificationData): Promise<boolean> {
    try {
      // In a real implementation, you would:
      // 1. Get the public key certificate from PayPal
      // 2. Verify the signature using the certificate
      // 3. Check transmission time is recent
      
      // For now, we'll do basic validation
      const requiredFields = [
        'authAlgorithm',
        'certId',
        'signature',
        'transmissionId',
        'transmissionTime',
        'webhookId'
      ];

      for (const field of requiredFields) {
        if (!data[field as keyof WebhookVerificationData]) {
          console.warn(`Missing required webhook field: ${field}`);
          return false;
        }
      }

      // Check transmission time is recent (within 5 minutes)
      const transmissionTime = new Date(data.transmissionTime).getTime();
      const now = Date.now();
      const fiveMinutes = 5 * 60 * 1000;
      
      if (Math.abs(now - transmissionTime) > fiveMinutes) {
        console.warn('Webhook transmission time is too old or in the future');
        return false;
      }

      // In development/testing, we can skip signature verification
      if (process.env.PAYPAL_MODE === 'sandbox' && process.env.NODE_ENV !== 'production') {
        console.log('Webhook signature verification skipped in sandbox mode');
        return true;
      }

      // TODO: Implement full signature verification for production
      console.warn('Full webhook signature verification not implemented - accepting all webhooks');
      return true;

    } catch (error) {
      console.error('Webhook verification error:', error);
      return false;
    }
  }

  /**
   * Extract webhook verification data from headers and body
   */
  extractWebhookData(headers: any, body: any): WebhookVerificationData {
    return {
      authAlgorithm: headers['paypal-auth-algo'] || '',
      certId: headers['paypal-cert-id'] || '',
      signature: headers['paypal-signature'] || '',
      transmissionId: headers['paypal-transmission-id'] || '',
      transmissionTime: headers['paypal-transmission-time'] || '',
      webhookId: process.env.PAYPAL_WEBHOOK_ID || '',
      webhookEvent: body
    };
  }

  /**
   * Validate webhook event structure
   */
  validateWebhookEvent(event: any): boolean {
    if (!event || typeof event !== 'object') {
      return false;
    }

    const requiredFields = ['id', 'event_type', 'create_time', 'resource_type'];
    
    for (const field of requiredFields) {
      if (!event[field]) {
        console.warn(`Missing required webhook event field: ${field}`);
        return false;
      }
    }

    return true;
  }

  /**
   * Generate a simple hash for idempotency checking
   */
  generateEventHash(event: any): string {
    const eventString = JSON.stringify({
      id: event.id,
      event_type: event.event_type,
      create_time: event.create_time,
      resource_id: event.resource?.id
    });
    
    return crypto.createHash('sha256').update(eventString).digest('hex');
  }
}

export const webhookVerifier = PayPalWebhookVerifier.getInstance();
