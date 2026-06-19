package cache

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type Store interface {
	Ping(ctx context.Context) error
	Increment(ctx context.Context, key string, ttl time.Duration) (int64, error)
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Del(ctx context.Context, key string) error
	Close() error
}

type RedisOptions struct {
	Addr         string
	Password     string
	DB           int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type RedisClient struct {
	addr         string
	password     string
	db           int
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func NewRedisClient(options RedisOptions) *RedisClient {
	return &RedisClient{
		addr:         firstNonEmpty(options.Addr, "127.0.0.1:6379"),
		password:     options.Password,
		db:           options.DB,
		dialTimeout:  defaultDuration(options.DialTimeout, 2*time.Second),
		readTimeout:  defaultDuration(options.ReadTimeout, 2*time.Second),
		writeTimeout: defaultDuration(options.WriteTimeout, 2*time.Second),
	}
}

func (c *RedisClient) Ping(ctx context.Context) error {
	value, err := c.command(ctx, "PING")
	if err != nil {
		return err
	}
	if strings.EqualFold(value.stringValue(), "PONG") {
		return nil
	}
	return fmt.Errorf("unexpected redis ping response: %s", value.stringValue())
}

func (c *RedisClient) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	value, err := c.command(ctx, "INCR", key)
	if err != nil {
		return 0, err
	}
	if ttl > 0 && value.integer == 1 {
		if _, err := c.command(ctx, "EXPIRE", key, strconv.FormatInt(int64(ttl.Seconds()), 10)); err != nil {
			return 0, err
		}
	}
	return value.integer, nil
}

func (c *RedisClient) Get(ctx context.Context, key string) (string, bool, error) {
	resp, err := c.command(ctx, "GET", key)
	if err != nil {
		return "", false, err
	}
	if resp.isNil {
		return "", false, nil
	}
	return resp.stringValue(), true, nil
}

func (c *RedisClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	args := []string{"SET", key, value}
	if ttl > 0 {
		if ttl < time.Second {
			args = append(args, "PX", strconv.FormatInt(ttl.Milliseconds(), 10))
		} else {
			args = append(args, "EX", strconv.FormatInt(int64(ttl.Seconds()), 10))
		}
	}
	resp, err := c.command(ctx, args...)
	if err != nil {
		return err
	}
	if !strings.EqualFold(resp.stringValue(), "OK") {
		return fmt.Errorf("unexpected redis set response: %s", resp.stringValue())
	}
	return nil
}

func (c *RedisClient) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	args := []string{"SET", key, value, "NX"}
	if ttl > 0 {
		if ttl < time.Second {
			args = append(args, "PX", strconv.FormatInt(ttl.Milliseconds(), 10))
		} else {
			args = append(args, "EX", strconv.FormatInt(int64(ttl.Seconds()), 10))
		}
	}
	resp, err := c.command(ctx, args...)
	if err != nil {
		return false, err
	}
	if resp.isNil {
		return false, nil
	}
	return strings.EqualFold(resp.stringValue(), "OK"), nil
}

func (c *RedisClient) Del(ctx context.Context, key string) error {
	_, err := c.command(ctx, "DEL", key)
	return err
}

func (c *RedisClient) Close() error {
	return nil
}

func (c *RedisClient) command(ctx context.Context, args ...string) (redisValue, error) {
	if len(args) == 0 {
		return redisValue{}, errors.New("redis command is empty")
	}
	conn, err := c.dial(ctx)
	if err != nil {
		return redisValue{}, err
	}
	defer conn.Close()

	if c.password != "" {
		if _, err := c.writeCommand(conn, "AUTH", c.password); err != nil {
			return redisValue{}, err
		}
		if _, err := readRedisValue(bufio.NewReader(conn)); err != nil {
			return redisValue{}, fmt.Errorf("redis auth: %w", err)
		}
	}
	if c.db > 0 {
		if _, err := c.writeCommand(conn, "SELECT", strconv.Itoa(c.db)); err != nil {
			return redisValue{}, err
		}
		if _, err := readRedisValue(bufio.NewReader(conn)); err != nil {
			return redisValue{}, fmt.Errorf("redis select db: %w", err)
		}
	}
	if _, err := c.writeCommand(conn, args...); err != nil {
		return redisValue{}, err
	}
	return readRedisValue(bufio.NewReader(conn))
}

func (c *RedisClient) dial(ctx context.Context) (net.Conn, error) {
	dialer := net.Dialer{Timeout: c.dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return nil, fmt.Errorf("dial redis %s: %w", c.addr, err)
	}
	return conn, nil
}

func (c *RedisClient) writeCommand(conn net.Conn, args ...string) (int, error) {
	if c.writeTimeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}
	var builder strings.Builder
	builder.WriteString("*")
	builder.WriteString(strconv.Itoa(len(args)))
	builder.WriteString("\r\n")
	for _, arg := range args {
		builder.WriteString("$")
		builder.WriteString(strconv.Itoa(len(arg)))
		builder.WriteString("\r\n")
		builder.WriteString(arg)
		builder.WriteString("\r\n")
	}
	written, err := io.WriteString(conn, builder.String())
	if err != nil {
		return written, fmt.Errorf("write redis command: %w", err)
	}
	if c.readTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(c.readTimeout))
	}
	return written, nil
}

type redisValue struct {
	typ     byte
	str     string
	integer int64
	isNil   bool
}

func (v redisValue) stringValue() string {
	if v.str != "" {
		return v.str
	}
	if v.typ == ':' {
		return strconv.FormatInt(v.integer, 10)
	}
	return ""
}

func readRedisValue(reader *bufio.Reader) (redisValue, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return redisValue{}, err
	}
	switch prefix {
	case '+':
		line, err := readRedisLine(reader)
		return redisValue{typ: prefix, str: line}, err
	case '-':
		line, err := readRedisLine(reader)
		if err != nil {
			return redisValue{}, err
		}
		return redisValue{}, errors.New(line)
	case ':':
		line, err := readRedisLine(reader)
		if err != nil {
			return redisValue{}, err
		}
		integer, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return redisValue{}, fmt.Errorf("parse redis integer %q: %w", line, err)
		}
		return redisValue{typ: prefix, integer: integer}, nil
	case '$':
		line, err := readRedisLine(reader)
		if err != nil {
			return redisValue{}, err
		}
		length, err := strconv.Atoi(line)
		if err != nil {
			return redisValue{}, fmt.Errorf("parse redis bulk length %q: %w", line, err)
		}
		if length < 0 {
			return redisValue{typ: prefix, isNil: true}, nil
		}
		buf := make([]byte, length+2)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return redisValue{}, err
		}
		return redisValue{typ: prefix, str: string(buf[:length])}, nil
	default:
		return redisValue{}, fmt.Errorf("unsupported redis response prefix %q", prefix)
	}
}

func readRedisLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

func defaultDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
