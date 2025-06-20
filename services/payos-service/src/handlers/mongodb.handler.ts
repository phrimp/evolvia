import { MongoClient, Db, Collection, ObjectId } from 'mongodb';

export interface Transaction {
  _id?: ObjectId;
  userId: string;
  orderCode: string;
  amount?: number;
  description?: string;
  status?: string;
  checkoutUrl?: string;
  createdAt?: Date;
  updatedAt?: Date;
}

class MongoDBHandler {
  private client: MongoClient;
  private db: Db;
  private transactionCollection: Collection<Transaction>;
  private isConnected: boolean = false;

  constructor() {
    const mongoUrl = process.env.MONGO_URI || 'mongodb://localhost:27017';
    this.client = new MongoClient(mongoUrl);
    this.db = this.client.db('payos_service');
    this.transactionCollection = this.db.collection<Transaction>('Transaction');
  }

  async connect(): Promise<void> {
    try {
      if (!this.isConnected) {
        await this.client.connect();
        this.isConnected = true;
        console.log('✅ Connected to MongoDB');
      }
    } catch (error) {
      console.error('❌ MongoDB connection error:', error);
      throw error;
    }
  }

  async disconnect(): Promise<void> {
    try {
      if (this.isConnected) {
        await this.client.close();
        this.isConnected = false;
        console.log('✅ Disconnected from MongoDB');
      }
    } catch (error) {
      console.error('❌ MongoDB disconnection error:', error);
    }
  }

  async createTransaction(transaction: Omit<Transaction, '_id'>): Promise<Transaction> {
    try {
      await this.connect();
      
      const newTransaction: Transaction = {
        ...transaction,
        createdAt: new Date(),
        updatedAt: new Date(),
      };

      const result = await this.transactionCollection.insertOne(newTransaction);
      
      return {
        ...newTransaction,
        _id: result.insertedId,
      };
    } catch (error) {
      console.error('❌ Error creating transaction:', error);
      throw error;
    }
  }

  async getTransactionsByUserId(userId: string): Promise<Transaction[]> {
    try {
      await this.connect();
      
      const transactions = await this.transactionCollection
        .find({ userId })
        .sort({ createdAt: -1 })
        .toArray();
      
      return transactions;
    } catch (error) {
      console.error('❌ Error getting transactions by userId:', error);
      throw error;
    }
  }

  async getTransactionByOrderCode(orderCode: string): Promise<Transaction | null> {
    try {
      await this.connect();
      
      const transaction = await this.transactionCollection.findOne({ orderCode });
      
      return transaction;
    } catch (error) {
      console.error('❌ Error getting transaction by orderCode:', error);
      throw error;
    }
  }

  async updateTransactionStatus(orderCode: string, status: string, additionalData?: Partial<Transaction>): Promise<Transaction | null> {
    try {
      await this.connect();
      
      const updateData: Partial<Transaction> = {
        status,
        updatedAt: new Date(),
        ...additionalData,
      };

      const result = await this.transactionCollection.findOneAndUpdate(
        { orderCode },
        { $set: updateData },
        { returnDocument: 'after' }
      );
      
      return result;
    } catch (error) {
      console.error('❌ Error updating transaction status:', error);
      throw error;
    }
  }

  async deleteTransaction(orderCode: string): Promise<boolean> {
    try {
      await this.connect();
      
      const result = await this.transactionCollection.deleteOne({ orderCode });
      
      return result.deletedCount > 0;
    } catch (error) {
      console.error('❌ Error deleting transaction:', error);
      throw error;
    }
  }
}

export const mongoDBHandler = new MongoDBHandler();