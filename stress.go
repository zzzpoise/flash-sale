//go:build ignore

package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	// === 压测参数配置 ===
	totalRequests := 10000 // 总共发 10000 个请求
	concurrency := 1000    // 并发协程数（同时有 1000 个人在抢）
	url := "http://localhost:8080/seckill"

	fmt.Printf("🚀 开始压测! 总请求: %d, 并发数: %d\n", totalRequests, concurrency)

	// 1. 创建一个任务通道（把 10000 个请求分发给协程）
	jobs := make(chan int, totalRequests)
	for i := 0; i < totalRequests; i++ {
		jobs <- i
	}
	close(jobs)

	// 用于统计的变量（使用原子操作保证并发安全）
	var successCount int64
	var failCount int64

	var wg sync.WaitGroup // 等待所有协程执行完毕

	startTime := time.Now()

	// 2. 开启 1000 个 Goroutine 一起干活
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 每个协程不断从 jobs 通道里拿任务，直到没任务为止
			for range jobs {
				// 模拟不同用户（使用随机数或者索引）
				// 注意：为了测试防黄牛，我们把 user_id 用随机数，控制在 1-2000 之间。
				// 这里为了简单，直接构造请求
				req, _ := http.NewRequest("POST", url+"?user_id=user_"+fmt.Sprint(rand.Intn(2000)), nil)

				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					continue
				}
				resp.Body.Close()

				if resp.StatusCode == 200 {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}
		}()
	}

	wg.Wait() // 挂起等待，直到 1000 个协程全部跑完
	duration := time.Since(startTime)

	// 3. 打印压测结果
	fmt.Println("====================================")
	fmt.Printf("✅ 压测完成！总耗时: %v\n", duration)
	fmt.Printf("📊 QPS (每秒处理请求数): %.2f\n", float64(totalRequests)/duration.Seconds())
	fmt.Printf("📈 网络请求成功: %d 次\n", successCount)
	fmt.Printf("📉 网络请求失败: %d 次\n", failCount)
	fmt.Println("====================================")
}

/*

package main

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	// === 压测参数配置 ===
	totalRequests := 10000 // 总共发 10000 个请求
	concurrency := 1000    // 并发协程数（同时有 1000 个人在抢）
	url := "http://localhost:8080/seckill"

	fmt.Printf("🚀 开始压测! 总请求: %d, 并发数: %d\n", totalRequests, concurrency)

	// 1. 创建一个任务通道（把 10000 个请求分发给协程）
	jobs := make(chan int, totalRequests)
	for i := 0; i < totalRequests; i++ {
		jobs <- i
	}
	close(jobs)

	// 用于统计的变量（使用原子操作保证并发安全）
	var successCount int64
	var failCount int64

	var wg sync.WaitGroup // 等待所有协程执行完毕

	startTime := time.Now()

	// 2. 开启 1000 个 Goroutine 一起干活
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 每个协程不断从 jobs 通道里拿任务，直到没任务为止
			for range jobs {
				// 模拟不同用户（使用随机数或者索引）
				// 注意：为了测试防黄牛，我们把 user_id 用随机数，控制在 1-2000 之间。
				// 这里为了简单，直接构造请求
				req, _ := http.NewRequest("POST", url+"?user_id=user_"+fmt.Sprint(rand.Intn(2000)), nil)

				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					continue
				}
				resp.Body.Close()

				if resp.StatusCode == 200 {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}
		}()
	}

	wg.Wait() // 挂起等待，直到 1000 个协程全部跑完
	duration := time.Since(startTime)

	// 3. 打印压测结果
	fmt.Println("====================================")
	fmt.Printf("✅ 压测完成！总耗时: %v\n", duration)
	fmt.Printf("📊 QPS (每秒处理请求数): %.2f\n", float64(totalRequests)/duration.Seconds())
	fmt.Printf("📈 网络请求成功: %d 次\n", successCount)
	fmt.Printf("📉 网络请求失败: %d 次\n", failCount)
	fmt.Println("====================================")
}
*/
