// +build !windows

package logger

import (
	"fmt"
	"go.uber.org/zap/zapcore"
	"os"
	"os/signal"
	"syscall"
)

func logLevelSignal() {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGUSR1, syscall.SIGUSR2)
	fmt.Println("notify receive signal")

	go func() {
		for s := range c {
			fmt.Println("receive signal ", s.String())
			switch s {
			case syscall.SIGUSR1:
				cur := atomicLevel.Level()
				if (cur - 1) >= zapcore.DebugLevel {
					atomicLevel.SetLevel(zapcore.Level(cur - 1))
				}
			case syscall.SIGUSR2:
				cur := atomicLevel.Level()
				if (cur + 1) <= zapcore.FatalLevel {
					atomicLevel.SetLevel(zapcore.Level(cur + 1))
				}
			default:
			}

			fmt.Println("debug level change to ", atomicLevel.String())
		}
	}()
}
