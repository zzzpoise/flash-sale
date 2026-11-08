package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// 将 Lua 脚本嵌入到 Go 二进制文件中
//
//go:embed lua/seckill.lua
var seckillScript string

// 全局变量：声明在函数外，供 main 和 Worker 共同使用
var ctx = context.Background()
var rdb *redis.Client

// 订单模型（以 JSON Lines 格式持久化到文件）
type Order struct {
	ID        uint      `json:"id,omitempty"`
	UserID    string    `json:"user_id"`
	ProductID string    `json:"product_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// 队列里的消息体（发往 Redis 队列的结构）
type OrderMessage struct {
	UserID    string
	ProductID string
}

func main() {
	// ================= 1. 初始化 Redis =================
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis默认端口
		Password: "",               // 没密码
		DB:       0,                // 默认数据库
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Redis 连接失败: ", err)
	}
	fmt.Println("✓ 成功连接 Redis")

	// ================= 2. 初始化秒杀库存 =================
	// 假设商品ID为1001的库存有 5 个
	// (如果你测试的时候发现没库存了，把这里的 5 改大一点，或者重启前删掉这个 key)
	rdb.Set(ctx, "seckill:stock:1001", 5, 0)

	// ================= 3. 启动后台异步 Worker =================
	go startOrderWorker()

	// ================= 4. 启动 Gin Web 服务 =================
	// 加载 Lua 脚本对象
	script := redis.NewScript(seckillScript)
	r := gin.Default()

	// 秒杀接口
	r.POST("/seckill", func(c *gin.Context) {
		// 获取用户ID，如果没传默认是 anonymous
		userID := c.Query("user_id")
		if userID == "" {
			userID = "anonymous"
		}

		key := "seckill:stock:1001"
		userSetKey := "seckill:users:1001" // 记录已经买过该商品的用户集合

		// 执行 Lua 脚本，原子操作
		// 传入 KEYS: [库存Key, 用户集合Key]
		// 传入 ARGV: [用户ID, 购买数量(1)]
		result, err := script.Run(ctx, rdb, []string{key, userSetKey}, userID, 1).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "系统错误"})
			return
		}

		// 解析 Lua 脚本返回的结果
		res := result.(int64)
		if res == 1 {
			// 【秒杀成功】：不直接写库，而是塞入 Redis List 队列，实现异步削峰
			msg := OrderMessage{UserID: userID, ProductID: "1001"}
			msgBytes, _ := json.Marshal(msg)
			rdb.LPush(ctx, "order:queue", msgBytes)

			c.JSON(http.StatusOK, gin.H{"message": "秒杀成功！正在排队生成订单"})
		} else if res == 0 {
			// 【库存不足】
			c.JSON(http.StatusOK, gin.H{"message": "手慢了，已被抢空"})
		} else if res == -2 {
			// 【重复购买】（防黄牛）
			c.JSON(http.StatusOK, gin.H{"message": "您已经抢到过了，请勿重复下单！"})
		} else {
			// 【商品不存在】
			c.JSON(http.StatusOK, gin.H{"message": "商品不存在"})
		}
	})

	fmt.Println("秒杀服务已启动在 :8080")
	r.Run("0.0.0.0:8080")
}

// ================= 后台异步消费者 Worker =================
func startOrderWorker() {
	fmt.Println("✓ 订单异步处理 Worker 已启动...")
	for {
		// BRPOP 是阻塞读取命令，如果队列为空，它会一直挂起等待
		// 参数 0 表示永远不超时，直到有数据塞进来
		res, err := rdb.BRPop(ctx, 0, "order:queue").Result()
		if err != nil {
			log.Println("读取队列失败:", err)
			continue
		}

		// res 是一个数组：res[0] 是队列名 "order:queue"，res[1] 才是真正的消息内容
		var msg OrderMessage
		if err := json.Unmarshal([]byte(res[1]), &msg); err != nil {
			continue
		}

		// 将订单追加写入本地 JSON Lines 文件，避免依赖未安装的 SQLite 驱动。
		order := Order{
			UserID:    msg.UserID,
			ProductID: msg.ProductID,
			Status:    "已支付",
			CreatedAt: time.Now(),
		}
		file, err := os.OpenFile("orders.jsonl", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Println("打开订单文件失败:", err)
			continue
		}
		if err := json.NewEncoder(file).Encode(order); err != nil {
			log.Println("写入订单失败:", err)
			file.Close()
			continue
		}
		if err := file.Close(); err != nil {
			log.Println("关闭订单文件失败:", err)
			continue
		}
		log.Printf("📦 异步入库成功: 用户 %s 购买了商品 %s\n", msg.UserID, msg.ProductID)
	}
}
