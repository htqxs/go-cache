package gocache

import (
	"fmt"
	"log"
	"sync"
)

// A Group is a cache namespace and associated data loaded spread over
// 一个 Group 可以认为是一个缓存的命名空间，每个 Group 拥有一个唯一的名称 name
type Group struct {
	name      string // group 的唯一名称
	getter    Getter // 缓存未命中时获取源数据的回调
	mainCache cache  // 并发缓存
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

// NewGroup create a new instance of Group
// 实例化 Group, 并且将 group 存储在全局变量 groups 中
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
	}
	groups[name] = g
	return g
}

// GetGroup returns the named group previously created with NewGroup, or
// nil if there's no such group.
func GetGroup(name string) *Group {
	mu.RLock() // 只读锁, 因为不涉及任何冲突变量的写操作
	g := groups[name]
	mu.RUnlock()
	return g
}

// Get value for a key from cache
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	// 1. 从 mainCache 查询 key, 若存在, 则返回值
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GeeCache] hit")
		return v, nil
	}
	// 2. 缓存不存在, 调用 load 方法获取源数据
	return g.load(key)
}

func (g *Group) load(key string) (value ByteView, err error) {
	return g.getLocally(key)
}

func (g *Group) getLocally(key string) (ByteView, error) {
	// 调用回调函数获取源数据
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err

	}
	// 获取成功则添加到缓存中
	value := ByteView{b: cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil
}

func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}

// A Getter loads data for a key.
// 回调 Getter，设计了一个回调函数(callback)，在缓存不存在时，调用此函数，得到源数据
type Getter interface {
	Get(key string) ([]byte, error)
}

// A GetterFunc implements Getter with a function.
// 定义函数类型 GetterFunc，并实现 Getter 接口的 Get 方法
type GetterFunc func(key string) ([]byte, error)

// Get implements Getter interface function
// 函数类型实现某一个接口，称之为接口型函数，方便使用者在调用时既能够传入函数作为参数，
// 也能够传入实现了该接口的结构体作为参数
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}