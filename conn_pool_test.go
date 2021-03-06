package freetds

import (
	"fmt"
	"github.com/stretchrcom/testify/assert"
	"testing"
	"time"
)

func TestPool(t *testing.T) {
	p, err := NewConnPool(testDbConnStr(), 2)
	defer p.Close()
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, len(p.pool), 1)
	c1, err := p.Get()
	assert.Nil(t, err)
	assert.NotNil(t, c1)
	assert.Equal(t, len(p.pool), 0)
	c2, err := p.Get()
	assert.Nil(t, err)
	assert.NotNil(t, c2)
	assert.Equal(t, len(p.pool), 0)
	p.Release(c1)
	assert.Equal(t, len(p.pool), 1)
	p.Release(c2)
	assert.Equal(t, len(p.pool), 2)
}

func TestPoolRelease(t *testing.T) {
	p, _ := NewConnPool(testDbConnStr(), 2)
	assert.Equal(t, p.connCount, 1)
	c1, _ := p.Get()
	c2, _ := p.Get()
	assert.Equal(t, p.connCount, 2)
	assert.Equal(t, len(p.pool), 0)
	//conn can be released to the pool by calling Close on conn
	c1.Close()
	assert.Equal(t, p.connCount, 2)
	assert.Equal(t, len(p.pool), 1)
	//or by calling pool Release
	p.Release(c2)
	assert.Equal(t, p.connCount, 2)
	assert.Equal(t, len(p.pool), 2)
}

func TestPoolBlock(t *testing.T) {
	p, _ := NewConnPool(testDbConnStr(), 2)
	c1, _ := p.Get()
	c2, _ := p.Get()

	//check that poolGuard channel is full
	full := false
	select {
	case p.poolGuard <- true:
	default:
		full = true
	}
	assert.True(t, full)

	go func() {
		c3, _ := p.Get()
		assert.Equal(t, c2, c3)
		c4, _ := p.Get()
		assert.Equal(t, c1, c4)
		p.Release(c3)
		p.Release(c4)
		p.Close()
	}()
	p.Release(c1)
	p.Release(c2)
}

func TestPoolCleanup(t *testing.T) {
	p, _ := NewConnPool(testDbConnStr(), 5)
	conns := make([]*Conn, 5)
	for i := 0; i < 5; i++ {
		c, _ := p.Get()
		conns[i] = c
	}
	for i := 0; i < 5; i++ {
		c := conns[i]
		p.Release(c)
		c.expiresFromPool = time.Now().Add(-poolExpiresInterval - time.Second)
	}
	assert.Equal(t, len(p.pool), 5)
	p.cleanup()
	assert.Equal(t, len(p.pool), 1)
}

func TestPoolReturnsLastUsedConnection(t *testing.T) {
	p, _ := NewConnPool(testDbConnStr(), 5)
	c1, _ := p.Get()
	c2, _ := p.Get()
	assert.Equal(t, 0, len(p.pool))
	c1.Close()
	assert.Equal(t, 1, len(p.pool))
	c2.Close()
	assert.Equal(t, 2, len(p.pool))
	assert.Equal(t, c2, p.pool[0])
	assert.Equal(t, c1, p.pool[1])
	c3, _ := p.Get()
	assert.Equal(t, c2, c3)
}

func BenchmarkConnPool(b *testing.B) {
	p, _ := NewConnPool(testDbConnStr(), 4)
	defer p.Close()
	done := make(chan bool)
	repeat := 20
	fmt.Printf("\n")
	for i := 0; i < repeat; i++ {
		go func(j int) {
			conn, _ := p.Get()
			defer p.Release(conn)
			fmt.Printf("running: %d pool len: %d, connCount %d\n", j, len(p.pool), p.connCount)
			conn.Exec("WAITFOR DELAY '00:00:01'")
			done <- true
		}(i)
	}
	for i := 0; i < repeat; i++ {
		<-done
	}
}
