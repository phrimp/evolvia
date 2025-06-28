import { MongoClient, Db, Collection, ObjectId } from 'mongodb';

export interface Transaction {
  _id?: ObjectId;
  userId: string;
  orderCode: string;
  amount?: number;
  description?: string;
  status?: string;
  checkoutUrl?: string;
  subscriptionID?: string | null;
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
        console.log('🔌 Attempting to connect to MongoDB...');
        await this.client.connect();
        // Add ping to verify connection
        await this.db.admin().ping();
        this.isConnected = true;
        console.log('✅ Connected to MongoDB successfully');
      } else {
        console.log('✅ MongoDB already connected');
      }
    } catch (error) {
      console.error('❌ MongoDB connection error:', error);
      console.error('❌ MongoDB connection error details:', error instanceof Error ? error.message : String(error));
      this.isConnected = false;
      throw error; // Throw error so calling code knows connection failed
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
      console.log('💾 Starting createTransaction...');
      console.log('💾 Input transaction data:', JSON.stringify(transaction, null, 2));
      
      await this.connect();
      
      const now = new Date();
      const newTransaction: Transaction = {
        ...transaction,
        status: transaction.status || 'PENDING_PAYMENT',
        subscriptionID: transaction.subscriptionID || null,
        createdAt: now,
        updatedAt: now,
      };

      console.log('💾 Creating transaction with data:', JSON.stringify(newTransaction, null, 2));
      
      const result = await this.transactionCollection.insertOne(newTransaction);
      
      const insertedTransaction = {
        ...newTransaction,
        _id: result.insertedId,
      };
      
      console.log('💾 Transaction inserted successfully with ID:', result.insertedId);
      console.log('💾 Full inserted transaction:', JSON.stringify(insertedTransaction, null, 2));
      
      return insertedTransaction;
    } catch (error) {
      console.error('❌ Error creating transaction:', error);
      console.error('❌ Error details:', error instanceof Error ? error.message : String(error));
      console.error('❌ Error stack:', error instanceof Error ? error.stack : 'No stack trace');
      throw error;
    }
  }

  async getTransactionsByUserId(userId: string): Promise<Transaction[]> {
    try {
      await this.connect();
      
      const transactions = await this.transactionCollection
        .find({ userId })
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

  async updateTransaction(orderCode: string, updateData: Partial<Transaction>): Promise<Transaction | null> {
    try {
      await this.connect();

      const result = await this.transactionCollection.findOneAndUpdate(
        { orderCode },
        { $set: updateData },
        { returnDocument: 'after' }
      );
      
      return result;
    } catch (error) {
      console.error('❌ Error updating transaction:', error);
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