import { Client, Environment } from '@paypal/paypal-server-sdk';

export interface PayPalConfig {
  clientId: string;
  clientSecret: string;
  mode: 'sandbox' | 'production';
}

export const paypalConfig: PayPalConfig = {
  clientId: process.env.PAYPAL_CLIENT_ID || '',
  clientSecret: process.env.PAYPAL_CLIENT_SECRET || '',
  mode: (process.env.PAYPAL_MODE as 'sandbox' | 'production') || 'sandbox'
};

export const getPayPalClient = (): Client => {
  const environment = paypalConfig.mode === 'production' 
    ? Environment.Production 
    : Environment.Sandbox;

  return new Client({
    clientCredentialsAuthCredentials: {
      oAuthClientId: paypalConfig.clientId,
      oAuthClientSecret: paypalConfig.clientSecret,
    },
    timeout: 0,
    environment: environment,
  });
};
