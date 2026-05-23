package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	MongoClient *mongo.Client
	MongoDB     *mongo.Database
)

// HashPassword 采用 SHA256 加盐进行密码哈希
func HashPassword(password string) string {
	hasher := sha256.New()
	hasher.Write([]byte(password + "_ikuai_salt_s9f2h8"))
	return hex.EncodeToString(hasher.Sum(nil))
}

// -------------------------------------------------------------
// 数据库模型定义
// -------------------------------------------------------------

type Hotel struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	HotelID     int32              `bson:"hotelId" json:"hotelId"`
	Name        string             `bson:"name" json:"name"`
	GatewayType string             `bson:"gateway_type" json:"gateway_type"`
	AppKey      string             `bson:"app_key" json:"app_key"`
	CustomName  string             `bson:"custom_name" json:"custom_name"`
	WelcomeText string             `bson:"welcome_text" json:"welcome_text"`
	Status      int32              `bson:"status" json:"status"`
	User        int64              `bson:"user" json:"user"`
	SMSCooldown int32              `bson:"sms_cooldown" json:"sms_cooldown"`
	IPCooldown  int32              `bson:"ip_cooldown" json:"ip_cooldown"`
	MaxSendsDay int32              `bson:"max_sends_day" json:"max_sends_day"`
	BypassAuth  int32              `bson:"bypass_auth" json:"bypass_auth"` // 0: 禁用免短信认证, 1: 启用免短信认证
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
}

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	User      int64              `bson:"user" json:"user"`
	Password  string             `bson:"password" json:"password,omitempty"`
	Level     int32              `bson:"level" json:"level"`
	Balance   int64              `bson:"balance" json:"balance"` // 单位: 分
	SMSCount  int32              `bson:"sms_count" json:"sms_count"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

type Package struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	PackageID string             `bson:"packageId" json:"packageId"`
	Name      string             `bson:"name" json:"name"`
	Price     int64              `bson:"price" json:"price"` // 单位: 分
	SMSCount  int32              `bson:"sms_count" json:"sms_count"`
	Status    int32              `bson:"status" json:"status"` // 1:启用, 0:禁用
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

type Recharge struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OrderID     string             `bson:"orderId" json:"orderId"` // ULID 格式
	User        int64              `bson:"user" json:"user"`
	Type        string             `bson:"type" json:"type"` // "package" | "balance"
	Amount      int64              `bson:"amount" json:"amount"` // 单位: 分
	SMSCount    int32              `bson:"sms_count" json:"sms_count"`
	PackageName string             `bson:"package_name" json:"package_name"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
}

type AuthLog struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	HotelID   int32              `bson:"hotelId" json:"hotelId"`
	Phone     string             `bson:"phone" json:"phone"`
	MAC       string             `bson:"mac" json:"mac"`
	IP        string             `bson:"ip" json:"ip"`
	Status    string             `bson:"status" json:"status"` // "success" | "failed"
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

type SMSLog struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	HotelID        int32              `bson:"hotelId" json:"hotelId"`
	User           int64              `bson:"user" json:"user"`
	Phone          string             `bson:"phone" json:"phone"`
	IP             string             `bson:"ip" json:"ip"`
	Code           string             `bson:"code" json:"code"`
	BillingType    string             `bson:"billing_type" json:"billing_type"` // "package" | "balance" | "free"
	DeductedCount  int32              `bson:"deducted_count" json:"deducted_count"`
	DeductedAmount int64              `bson:"deducted_amount" json:"deducted_amount"` // 单位: 分
	Provider       string             `bson:"provider" json:"provider"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
}

type SMSProvider struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Provider  string             `bson:"provider" json:"provider"` // "aliyun" | "tencent" | "mock"
	Weight    int32              `bson:"weight" json:"weight"`
	Config    map[string]string  `bson:"config" json:"config"`
	Status    int32              `bson:"status" json:"status"` // 1:启用, 0:禁用
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// -------------------------------------------------------------
// 数据库连接与初始化
// -------------------------------------------------------------

func InitMongoDB(ctx context.Context, uri, dbName string) error {
	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return fmt.Errorf("MongoDB 连接失败: %v", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return fmt.Errorf("MongoDB Ping 失败: %v", err)
	}

	MongoClient = client
	MongoDB = client.Database(dbName)

	log.Println("🔌 MongoDB 连接成功")

	// 1. 创建高并发查询索引
	if err := createIndices(ctx); err != nil {
		return fmt.Errorf("创建索引失败: %v", err)
	}

	// 2. 插入初始化种子数据
	if err := seedDatabase(ctx); err != nil {
		return fmt.Errorf("初始化种子数据失败: %v", err)
	}

	return nil
}

func createIndices(ctx context.Context) error {
	createOptIndex := func(collName string, keys bson.D, unique bool) error {
		coll := MongoDB.Collection(collName)
		opts := options.Index().SetUnique(unique)
		model := mongo.IndexModel{
			Keys:    keys,
			Options: opts,
		}
		_, err := coll.Indexes().CreateOne(ctx, model)
		if err != nil {
			return fmt.Errorf("集合 %s 索引创建失败: %v", collName, err)
		}
		return nil
	}

	// hotels 集合索引
	if err := createOptIndex("hotels", bson.D{{Key: "hotelId", Value: 1}}, true); err != nil {
		return err
	}
	if err := createOptIndex("hotels", bson.D{{Key: "user", Value: 1}}, false); err != nil {
		return err
	}

	// users 集合索引
	if err := createOptIndex("users", bson.D{{Key: "user", Value: 1}}, true); err != nil {
		return err
	}

	// packages 集合索引
	if err := createOptIndex("packages", bson.D{{Key: "packageId", Value: 1}}, true); err != nil {
		return err
	}
	if err := createOptIndex("packages", bson.D{{Key: "status", Value: 1}}, false); err != nil {
		return err
	}

	// recharge 集合索引
	if err := createOptIndex("recharge", bson.D{{Key: "orderId", Value: 1}}, true); err != nil {
		return err
	}
	if err := createOptIndex("recharge", bson.D{{Key: "user", Value: 1}}, false); err != nil {
		return err
	}
	if err := createOptIndex("recharge", bson.D{{Key: "created_at", Value: -1}}, false); err != nil {
		return err
	}

	// auth_logs 集合索引
	if err := createOptIndex("auth_logs", bson.D{{Key: "hotelId", Value: 1}, {Key: "created_at", Value: -1}}, false); err != nil {
		return err
	}
	if err := createOptIndex("auth_logs", bson.D{{Key: "phone", Value: 1}}, false); err != nil {
		return err
	}
	if err := createOptIndex("auth_logs", bson.D{{Key: "created_at", Value: -1}}, false); err != nil {
		return err
	}

	// sms_logs 集合索引
	if err := createOptIndex("sms_logs", bson.D{{Key: "hotelId", Value: 1}, {Key: "created_at", Value: -1}}, false); err != nil {
		return err
	}
	if err := createOptIndex("sms_logs", bson.D{{Key: "phone", Value: 1}}, false); err != nil {
		return err
	}

	// sms_providers 集合索引
	if err := createOptIndex("sms_providers", bson.D{{Key: "status", Value: 1}}, false); err != nil {
		return err
	}

	log.Println("⚡ 数据库索引初始化/检查完成")
	return nil
}

func seedDatabase(ctx context.Context) error {
	// 1. 初始化计数器
	countersColl := MongoDB.Collection("counters")
	var counterDoc bson.M
	err := countersColl.FindOne(ctx, bson.M{"_id": "hotelId"}).Decode(&counterDoc)
	if err == mongo.ErrNoDocuments {
		_, err = countersColl.InsertOne(ctx, bson.M{"_id": "hotelId", "seq": int32(1000000)})
		if err != nil {
			return err
		}
		log.Println("🌱 初始化 hotelId 自增长计数器")
	}

	// 2. 初始化管理员账号 (如果系统内没有 level = 100 的超级管理员)
	usersColl := MongoDB.Collection("users")
	var adminDoc User
	err = usersColl.FindOne(ctx, bson.M{"level": int32(100)}).Decode(&adminDoc)
	if err == mongo.ErrNoDocuments {
		adminUser := User{
			User:      13703770377, // 预置管理员数字账号
			Password:  HashPassword("aa123456"),
			Level:     100,
			Balance:   99999900, // 初始预置金额
			SMSCount:  999999,
			CreatedAt: time.Now(),
		}
		_, err = usersColl.InsertOne(ctx, adminUser)
		if err != nil {
			return err
		}
		log.Println("🌱 初始化超级管理员账号: 13703770377 (密码 aa123456)")
	}

	// 3. 初始化默认测试商户 (如果系统内没有该商户)
	var merchantDoc User
	err = usersColl.FindOne(ctx, bson.M{"user": int64(13803770377)}).Decode(&merchantDoc)
	if err == mongo.ErrNoDocuments {
		merchantUser := User{
			User:      13803770377,
			Password:  HashPassword("123456"),
			Level:     10,
			Balance:   15050, // 150.50 元
			SMSCount:  500,   // 500 条
			CreatedAt: time.Now(),
		}
		_, err = usersColl.InsertOne(ctx, merchantUser)
		if err != nil {
			return err
		}
		log.Println("🌱 初始化商户账号: 13803770377 (密码 123456)")
	}

	// 4. 初始化默认充值套餐 (如果集合为空)
	pkgColl := MongoDB.Collection("packages")
	count, err := pkgColl.EstimatedDocumentCount(ctx)
	if err == nil && count == 0 {
		pkgs := []Package{
			{PackageID: "pkg_01", Name: "经典商务套餐包", Price: 5000, SMSCount: 1000, Status: 1, CreatedAt: time.Now()},
			{PackageID: "pkg_02", Name: "黄金大容量套餐", Price: 10000, SMSCount: 2200, Status: 1, CreatedAt: time.Now()},
			{PackageID: "pkg_03", Name: "至尊无限运营包", Price: 20000, SMSCount: 5000, Status: 1, CreatedAt: time.Now()},
		}
		for _, p := range pkgs {
			_, err = pkgColl.InsertOne(ctx, p)
			if err != nil {
				return err
			}
		}
		log.Println("🌱 初始化 3 档经典短信充值套餐")
	}

	// 5. 初始化 Mock 短信发送通道 (如果为空)
	providersColl := MongoDB.Collection("sms_providers")
	count, err = providersColl.EstimatedDocumentCount(ctx)
	if err == nil && count == 0 {
		mockProvider := SMSProvider{
			Provider:  "mock",
			Weight:    100,
			Status:    1,
			Config:    map[string]string{"sign_name": "系统统一认证"},
			CreatedAt: time.Now(),
		}
		_, err = providersColl.InsertOne(ctx, mockProvider)
		if err != nil {
			return err
		}
		log.Println("🌱 初始化本地 Mock 短信发送通道 (默认权重 100)")
	}

	return nil
}

// -------------------------------------------------------------
// 数据库核心业务操作封装
// -------------------------------------------------------------

// GenerateNextHotelID 自动生成自增 hotelId
func GenerateNextHotelID(ctx context.Context) (int32, error) {
	coll := MongoDB.Collection("counters")
	filter := bson.M{"_id": "hotelId"}
	update := bson.M{"$inc": bson.M{"seq": 1}}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var result struct {
		Seq int32 `bson:"seq"`
	}
	err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		return 0, err
	}
	return result.Seq, nil
}

// GetHotelByHotelID 获取单个酒店信息
func GetHotelByHotelID(ctx context.Context, hotelId int32) (*Hotel, error) {
	coll := MongoDB.Collection("hotels")
	var h Hotel
	err := coll.FindOne(ctx, bson.M{"hotelId": hotelId}).Decode(&h)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &h, nil
}

// SaveAuthLog 记录认证放行日志
func SaveAuthLog(ctx context.Context, hotelID int32, phone, mac, ip, status string) {
	coll := MongoDB.Collection("auth_logs")
	logDoc := AuthLog{
		HotelID:   hotelID,
		Phone:     phone,
		MAC:       mac,
		IP:        ip,
		Status:    status,
		CreatedAt: time.Now(),
	}
	_, err := coll.InsertOne(ctx, logDoc)
	if err != nil {
		log.Printf("❌ 写入认证日志失败: %v", err)
	}
}

// SaveSMSLog 记录短信发送日志
func SaveSMSLog(ctx context.Context, hotelID int32, phone, ip, code, billingType string, count int32, amount int64, provider string, user int64) {
	coll := MongoDB.Collection("sms_logs")
	logDoc := SMSLog{
		HotelID:        hotelID,
		User:           user,
		Phone:          phone,
		IP:             ip,
		Code:           code,
		BillingType:    billingType,
		DeductedCount:  count,
		DeductedAmount: amount,
		Provider:       provider,
		CreatedAt:      time.Now(),
	}
	_, err := coll.InsertOne(ctx, logDoc)
	if err != nil {
		log.Printf("❌ 写入短信日志失败: %v", err)
	}
}
