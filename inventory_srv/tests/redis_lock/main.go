package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredislib "github.com/redis/go-redis/v9"
)

func main() {
	client := goredislib.NewClient(&goredislib.Options{
		Addr: "localhost:6379",
	})
	pool := goredis.NewPool(client)

	rs := redsync.New(pool)

	gNum := 2
	wg := sync.WaitGroup{}
	wg.Add(gNum)
	for i := 0; i < gNum; i++ {
		go func() {
			defer wg.Done()
			mutexname := "my-global-mutex"
			mutex := rs.NewMutex(mutexname)

			if err := mutex.Lock(); err != nil {
				panic(err)
			}
			fmt.Println("获取锁成功")
			time.Sleep(time.Second * 3)

			if ok, err := mutex.Unlock(); !ok || err != nil {
				panic("unlock failed")
			}
			fmt.Println("释放锁成功")
		}()
	}
	wg.Wait()
}
