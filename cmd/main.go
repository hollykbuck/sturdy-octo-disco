package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/consul/api"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const gitContextKey = "git"

type gitCommitOption struct {
	f              *os.File
	textFilePath   string
	repoDirTrimmed string
}

type gitContext struct {
	num          int
	verbose      bool
	tryForcePush bool
}

func gitCommit(option *gitCommitOption, ctx *gitContext) error {
	_, err := option.f.WriteString("random word\n")
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	gitAddCommand := exec.Command("git", "add", option.textFilePath)
	gitAddCommand.Stderr = os.Stderr
	gitAddCommand.Dir = option.repoDirTrimmed
	err = gitAddCommand.Run()
	if err != nil {
		return fmt.Errorf("执行 git add 失败: %w", err)
	}
	gitCommitCommand := exec.Command("git", "commit", "-m", "regular")
	if ctx.verbose {
		gitCommitCommand.Stdout = os.Stdout
	}
	gitCommitCommand.Stderr = os.Stderr
	gitCommitCommand.Dir = option.repoDirTrimmed
	err = gitCommitCommand.Run()
	if err != nil {
		return fmt.Errorf("执行 git commit 失败: %w", err)
	}
	return nil
}

func execGit(context context.Context) error {
	gitContextValue, ok := context.Value(gitContextKey).(*gitContext)
	if !ok {
		return fmt.Errorf("context 类型错误: %T", context.Value(gitContextKey))
	}
	repoDir := os.Getenv("HONEYDEW_REPO_DIR")
	repoDirTrimmed := strings.TrimSpace(repoDir)
	if repoDirTrimmed == "" {
		return fmt.Errorf("未设置 REPO 路径 HONEYDEW_REPO_DIR")
	}
	info, err := os.Stat(repoDirTrimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("目录 %s 不存在: %w", repoDirTrimmed, err)
		}
		return fmt.Errorf("目录 %s 打开失败: %w", repoDirTrimmed, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s 不是目录", repoDirTrimmed)
	}
	err = os.Chdir(repoDirTrimmed)
	if err != nil {
		return fmt.Errorf("cd %q 失败: %w", repoDirTrimmed, err)
	}
	textFilePath := filepath.Join(repoDir, "hello.txt")
	_, err = os.Stat(textFilePath)
	var f *os.File
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f, err = os.OpenFile(textFilePath, os.O_RDWR|os.O_CREATE, 0600)
			if err != nil {
				return fmt.Errorf("文件 %q 创建失败: %w", textFilePath, err)
			}

		} else {
			return err
		}
	} else {
		f, err = os.OpenFile(textFilePath, os.O_RDWR|os.O_APPEND, 0600)
		if err != nil {
			return fmt.Errorf("文件 %q 打开失败: %w", textFilePath, err)
		}
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
	}(f)
	for i := 0; i < gitContextValue.num; i++ {
		err = gitCommit(&gitCommitOption{
			f:              f,
			textFilePath:   textFilePath,
			repoDirTrimmed: repoDirTrimmed,
		}, gitContextValue)
		if err != nil {
			return err
		}
	}
	gitPushCommand := exec.Command("git", "push", "origin", "master:main")
	gitPushCommand.Dir = repoDirTrimmed
	gitPushCommand.Stderr = os.Stderr
	err = gitPushCommand.Run()
	normalPushWorked := true
	if err != nil {
		normalPushWorked = false
		if !gitContextValue.tryForcePush {
			return err
		}
	}
	if !normalPushWorked && gitContextValue.tryForcePush {
		gitForcePushCommand := exec.Command("git", "push", "origin", "master:main")
		gitForcePushCommand.Dir = repoDirTrimmed
		gitForcePushCommand.Stderr = os.Stderr
		err = gitForcePushCommand.Run()
		if err != nil {
			return fmt.Errorf("force push 失败: %w", err)
		}
	}
	return nil
}

func _main() error {
	background := context.Background()
	g := &gitContext{
		num:          0,
		verbose:      true,
		tryForcePush: true,
	}
	err := getConsulData(g)
	if err != nil {
		return err
	}
	gitCtx := context.WithValue(background, gitContextKey, g)
	HOME := os.Getenv("HOME")
	if HOME == "" {
		return fmt.Errorf("env HOME is not defined")
	}
	//err = execList(HOME)
	//if err != nil {
	//	return err
	//}
	err = execGit(gitCtx)
	if err != nil {
		return err
	}
	log.Println("执行结束")
	return nil
}

type ConsulData struct {
	Num int `json:"num"`
}

func getConsulData(ctx *gitContext) error {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return err
	}
	consulKV := client.KV()
	data, _, err := consulKV.Get("config/honeydew", nil)
	if err != nil {
		return err
	}
	value := data.Value
	consulData := ConsulData{}
	err = json.Unmarshal(value, &consulData)
	if err != nil {
		return err
	}
	if ctx.verbose {
		log.Printf("%#v", consulData)
	}
	ctx.num = consulData.Num
	return nil
}

func main() {
	err := _main()
	if err != nil {
		log.Println(err)
		return
	}
}
