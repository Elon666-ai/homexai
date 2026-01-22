package utils

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var runEnv string = "dev"

func init() {

	//环境变量 > .env 文件
	env, _ := GetEnvFromFile(ENV_FILE, "ENV")
	if os.Getenv("ENV") != "" {
		env = os.Getenv("ENV")
	}

	env = strings.TrimRight(env, "\r\n")
	env = strings.TrimSpace(env)
	env = strings.ToLower(env)
	if env != "prod" && env != "stag" && env != "uat" && env != "dev" && env != "test" && env != "local" {
		env = ""
	}
	runEnv = env

	go func() {
		ticker := time.NewTicker(CleanupInterval)
		for range ticker.C {
			rateLimitMap = sync.Map{}
		}
	}()
}

func GetRunEnv() string {
	return runEnv
}

func AppendResetLog() {
	f, err := os.OpenFile(RESET_LOGFILE, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0660)
	if err == nil {
		now := time.Now()
		log := fmt.Sprintf("%s %s restart at %s\n", APP_NAME, VERSION, now.Format("20060102150405"))
		f.WriteString(log)
		f.Close()
	}
}

func LogPid() {
	f, err := os.OpenFile(PID_LOGFILE, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0660)
	if err == nil {
		pid := syscall.Getpid()
		f.WriteString(fmt.Sprintf("%d", pid))
		f.Close()
	}
}

func GetPid() int {
	data, err := os.ReadFile(PID_LOGFILE)
	if err != nil {
		fmt.Println(PID_LOGFILE, "not found!")
		return 0
	}

	pid, _ := strconv.Atoi(string(data))
	return pid
}

// GetEnvFromFile 读取 .env 文件并返回指定 key 的值
func GetEnvFromFile(filename, key string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行或注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, key+"=") {
			value := strings.TrimPrefix(line, key+"=")
			return value, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", errors.New("key not found in file")
}
