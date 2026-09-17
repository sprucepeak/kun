
package {{ .PackageName }}

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"{{ .ProjectName }}/pkg/xredis"
)

var _ {{ .FileName }}Cache = (*{{ .FileNameTitleLower }}Cache)(nil)

type (
	{{ .FileName }}Cache interface {
		// 查询缓存
		Get(ctx context.Context, id int64) (*{{ .FileName }}, error)
		// 添加缓存
		Set(ctx context.Context, id int64, data *{{ .FileName }}, expiration int64) error
        // 删除缓存
        Delete(ctx context.Context, id int64) error
        // 预热缓存
        Warmup(ctx context.Context) error
        // TODO: add cache functions here and delete this line
	}

	{{ .FileNameTitleLower }}Cache struct {
		common     *xredis.Client
		localCache *LocalCache
	}

	//  缓存数据的结构体
	{{ .FileName }} struct {
        // TODO: add struct fields here and delete this line
	}

    // TODO: add struct here and delete this line
)

const (
	// {{ .FileNameTitleLower }}DefaultExpiration 当调用方未指定(<=0)TTL 时的默认缓存时长。
	// 避免传 0 时 Redis Set 永不过期,导致脏数据长期残留(需手动 Delete 才能失效)。
	{{ .FileNameTitleLower }}DefaultExpiration = 5 * time.Minute
)

func New{{ .FileName }}Cache(c *xredis.Client) {{ .FileName }}Cache {
	return &{{ .FileNameTitleLower }}Cache{
		common:     c,
		localCache: NewLocalCache(5*time.Minute, 10*time.Minute),
	}
}

func ({{ .FileNameFirstChar }} *{{ .FileNameTitleLower }}Cache) Get(ctx context.Context, id int64) (*{{ .FileName }}, error) {
	key := fmt.Sprintf({{ .FileName }}DataKey, id)

	// 1.查询本地缓存
	if value, ok := {{ .FileNameFirstChar }}.localCache.Get(key); ok {
		if {{ .FileNameTitleLower }}, ok := value.(*{{ .FileName }}); ok {
			// 返回副本,避免调用方修改污染缓存内对象
			cp := *{{ .FileNameTitleLower }}
			return &cp, nil
		}
	}

	// 2.查询Redis缓存
	{{ .FileNameTitleLower }} := &{{ .FileName }}{}
	data, err := {{ .FileNameFirstChar }}.common.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, xredis.Nil) {
			return nil, fmt.Errorf("cache miss: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("get cache failed: %w", err)
	}
	err = json.Unmarshal(data, {{ .FileNameTitleLower }})
	if err != nil {
		return nil, err
	}
	// 3.设置本地缓存
	{{ .FileNameFirstChar }}.localCache.Set(key, {{ .FileNameTitleLower }}, 5*time.Minute)
	// 返回副本,避免调用方修改污染缓存内对象
	cp := *{{ .FileNameTitleLower }}
	return &cp, nil
}

func ({{ .FileNameFirstChar }} *{{ .FileNameTitleLower }}Cache) Set(ctx context.Context, id int64, data *{{ .FileName }}, expiration int64) error {
	key := fmt.Sprintf({{ .FileName }}DataKey, id)

	value, valueErr := json.Marshal(data)
	if valueErr != nil {
		return valueErr
	}
	// expiration<=0 时用默认 TTL,避免 Redis key 永不过期导致脏数据残留。
	// 注意 go-redis 把 0 解释为"无 TTL",而 go-cache 把 0 解释为"用默认过期",
	// 两层语义不一致,必须在此统一。
	ttl := time.Duration(expiration) * time.Second
	if expiration <= 0 {
		ttl = {{ .FileNameTitleLower }}DefaultExpiration
	}
	err := {{ .FileNameFirstChar }}.common.Set(ctx, key, string(value), ttl).Err()
	if err != nil {
		return err
	}
	// 同步更新本地缓存,避免 Get 读到脏数据。
	// 存副本:直接存调用方的指针,调用方之后修改该对象会污染缓存内容。
	cp := *data
	{{ .FileNameFirstChar }}.localCache.Set(key, &cp, ttl)
	return nil
}

func ({{ .FileNameFirstChar }} *{{ .FileNameTitleLower }}Cache) Delete(ctx context.Context, id int64) error {
	key := fmt.Sprintf({{ .FileName }}DataKey, id)

	// 本地缓存是 Redis 的副本,无论 Redis 删除是否成功都先清本地,避免残留旧值。
	{{ .FileNameFirstChar }}.localCache.Delete(key)

	if err := {{ .FileNameFirstChar }}.common.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete cache failed: %w", err)
	}
	return nil
}


func ({{ .FileNameFirstChar }} *{{ .FileNameTitleLower }}Cache) Warmup(ctx context.Context) error {
	return nil
}

// TODO: add your code here and delete this line