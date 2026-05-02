package due

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/dobyte/due/v2/cache"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/lock"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xos"
)

const (
	defaultPIDKey                 = "etc.pid"                 // 进程文件路径
	defaultShutdownMaxWaitTimeKey = "etc.shutdownMaxWaitTime" // 容器关闭最大等待时间
)

type Container struct {
	components []component.Component
}

// NewContainer 创建一个容器
func NewContainer() *Container {
	return &Container{}
}

// Add 添加组件
func (c *Container) Add(components ...component.Component) {
	c.components = append(c.components, components...)
}

// Serve 启动容器
// 生命周期入口：Init → Start → [等待信号] → Close → Destroy → ClearModules
// once=true 时跳过信号等待，执行一次后立即关闭（用于测试）
func (c *Container) Serve(once ...bool) {
	c.doSaveProcessID()

	c.doPrintFrameworkInfo()

	// 顺序初始化：按 Add() 顺序执行，保证有依赖关系的组件按预期顺序启动
	c.doInitComponents()

	// 顺序启动：同上，顺序保证
	c.doStartComponents()

	if len(once) == 0 || !once[0] {
		// 阻塞直到收到 SIGINT/SIGTERM 等信号
		c.doWaitSystemSignal()
	}

	// 并发关闭：各组件互相独立，并发可加快退出速度；超时由 etc.shutdownMaxWaitTime 控制
	c.doCloseComponents()

	// 并发销毁：固定 5s 超时，超时后不再等待（goroutine 仍在运行，只是不等了）
	c.doDestroyComponents()

	// 顺序清理全局模块（eventbus/lock/cache/task/config/etc/log）
	c.doClearModules()
}

// 初始化所有组件
func (c *Container) doInitComponents() {
	for _, comp := range c.components {
		comp.Init()
	}
}

// 启动所有组件
func (c *Container) doStartComponents() {
	for _, comp := range c.components {
		comp.Start()
	}
}

// 关闭所有组件
func (c *Container) doCloseComponents() {
	g := xcall.NewGoroutines()

	for _, comp := range c.components {
		g.Add(comp.Close)
	}

	g.Run(context.Background(), etc.Get(defaultShutdownMaxWaitTimeKey).Duration())
}

// 销毁所有组件
func (c *Container) doDestroyComponents() {
	g := xcall.NewGoroutines()

	for _, comp := range c.components {
		g.Add(comp.Destroy)
	}

	g.Run(context.Background(), 5*time.Second)
}

// 等待系统信号
func (c *Container) doWaitSystemSignal() {
	sig := make(chan os.Signal)

	switch runtime.GOOS {
	case `windows`:
		signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
	default:
		signal.Notify(sig, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGKILL, syscall.SIGTERM)
	}

	s := <-sig

	signal.Stop(sig)

	log.Warnf("process got signal %v, container will close", s)
}

// 清理所有模块
func (c *Container) doClearModules() {
	if err := eventbus.Close(); err != nil {
		log.Warnf("eventbus close failed: %v", err)
	}

	if err := lock.Close(); err != nil {
		log.Warnf("lock-maker close failed: %v", err)
	}

	if err := cache.Close(); err != nil {
		log.Warnf("cache close failed: %v", err)
	}

	task.Release()

	config.Close()

	etc.Close()

	log.Close()
}

// 保存进程号
func (c *Container) doSaveProcessID() {
	filename := etc.Get(defaultPIDKey).String()
	if filename == "" {
		return
	}

	if err := xos.WriteFile(filename, []byte(strconv.Itoa(syscall.Getpid()))); err != nil {
		log.Fatalf("pid save failed: %v", err)
	}
}

// 打印框架信息
func (c *Container) doPrintFrameworkInfo() {
	info.PrintFrameworkInfo()

	info.PrintGlobalInfo()
}
