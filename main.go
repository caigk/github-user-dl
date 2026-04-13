package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// GitHub仓库结构体（仅保留核心字段）
type GitHubRepo struct {
	Name     string `json:"name"`
	CloneURL string `json:"clone_url"`
}

// 全局变量
var (
	username    string      // GitHub用户名
	targetPath  string      // 目标路径
	logFilePath string      // 日志文件路径
	logger      *log.Logger // 日志实例
)

func init() {
	// 解析命令行参数
	flag.StringVar(&username, "u", "", "GitHub用户名（必填）")
	flag.StringVar(&targetPath, "p", "", "目标路径（缺省为./{用户名}）")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "使用说明: github-user-dl -u <用户名> [-p <目标路径>]\n")
		flag.PrintDefaults()
	}
}

// ---------------------------------------------------------------------------------
// 状态管理和流程控制 (新增)
// ---------------------------------------------------------------------------------

const manifestFile = "processed_manifest.json"

// loadManifest 从状态文件中加载已处理的仓库集合
func loadManifest(targetDir string) (map[string]bool, error) {
	manifestPath := filepath.Join(targetDir, manifestFile)
	data, err := ioutil.ReadFile(manifestPath)
	if os.IsNotExist(err) {
		logger.Printf("未找到状态文件 %s，从头开始扫描所有仓库。", manifestFile)
		return make(map[string]bool), nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取状态文件失败: %w", err)
	}

	var manifest map[string]bool
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("解析状态文件失败，可能文件已损坏: %w", err)
	}
	logger.Printf("成功加载状态文件，已记录 %d 个已处理的仓库。", len(manifest))
	return manifest, nil
}

// saveManifest 将当前成功处理的仓库列表保存到状态文件
func saveManifest(targetDir string, processedRepos []GitHubRepo) error {
	manifestPath := filepath.Join(targetDir, manifestFile)

	// 1. 读取现有状态
	existingManifest, err := loadManifest(targetDir)
	if err != nil {
		logger.Printf("警告：无法加载现有状态文件，本次操作将覆盖或忽略状态：%v", err)
		existingManifest = make(map[string]bool)
	}

	// 2. 更新状态：将本次成功处理的仓库加入到已处理集合中
	for _, repo := range processedRepos {
		existingManifest[repo.Name] = true
	}

	// 3. 写入新状态
	data, err := json.MarshalIndent(existingManifest, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态文件失败: %w", err)
	}

	if err := ioutil.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("写入状态文件失败: %w", err)
	}
	logger.Printf("✅ 状态文件已更新：已记录 %d 个已处理的仓库。", len(existingManifest))
	return nil
}

// ---------------------------------------------------------------------------------
// 主函数入口
// ---------------------------------------------------------------------------------
func main() {
	// 解析参数
	flag.Parse()

	// 校验必填参数
	if username == "" {
		flag.Usage()
		os.Exit(1)
	}

	// 设置默认目标路径
	if targetPath == "" {
		targetPath = fmt.Sprintf("./%s", username)
	}

	// 初始化日志
	initLogger()

	// 1. 创建目标路径
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		fmt.Printf("创建目录失败: %v", err)
		return
	}

	// 2. 加载状态，实现断点续传
	processedManifest, err := loadManifest(targetPath)
	if err != nil {
		logger.Fatalf("致命错误：无法加载或解析状态文件，请检查日志。错误: %v", err)
	}

	// 循环重试直到所有仓库拉取成功
	for {
		// 2. 获取用户公开仓库列表
		repos, err := listPublicRepos(username)
		if err != nil {
			logger.Printf("获取仓库列表失败: %v，20秒后重试", err)
			time.Sleep(20 * time.Second)
			continue
		}

		if len(repos) == 0 {
			logger.Printf("未找到该用户的公开仓库")
			return
		}

		// 过滤出尚未处理的仓库
		var reposToProcess []GitHubRepo
		var allRepos []GitHubRepo
		for _, repo := range repos {
			allRepos = append(allRepos, repo)
			if !processedManifest[repo.Name] {
				reposToProcess = append(reposToProcess, repo)
			}
		}

		if len(reposToProcess) == 0 {
			logger.Printf("所有 %d 个仓库均已在状态文件中标记为已处理，任务完成。", len(allRepos))
			break
		}

		// 3. 批量拉取/更新仓库
		failedRepos := pullOrCloneRepos(reposToProcess)

		// 4. 校验所有仓库是否成功拉取
		validFailed := validateRepos(repos, failedRepos)
		if len(validFailed) == 0 {
			// 成功：更新状态文件，标记本次处理的仓库
			if err := saveManifest(targetPath, reposToProcess); err != nil {
				logger.Fatalf("无法保存状态文件，请检查权限。错误: %v", err)
			}
			logger.Printf("所有待处理仓库已成功拉取/更新完成。")
			break
		}

		// 5. 存在失败项，重试
		logger.Printf("以下仓库拉取失败: %v，10秒后重试", strings.Join(validFailed, ","))
		time.Sleep(10 * time.Second)
	}
}

// 初始化日志（写入目标路径的 github-dl.log 文件）
func initLogger() {
	logFilePath = filepath.Join(targetPath, "github-dl.log")

	// 创建日志目录（如果不存在）
	_ = os.MkdirAll(targetPath, 0755)

	// 打开日志文件（追加模式，不存在则创建）
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("创建日志文件失败: %v", err)
	}

	// 初始化日志实例（同时输出到控制台和文件）
	logger = log.New(os.Stdout, "", log.LstdFlags)
	logger.SetOutput(io.MultiWriter(os.Stdout, logFile))
}

// 获取用户公开仓库列表（通过GitHub Open API），包含速率限制重试逻辑
func listPublicRepos(user string) ([]GitHubRepo, error) {
	apiURL := fmt.Sprintf("https://api.github.com/users/%s/repos", user)
	logger.Printf("正在获取仓库列表: %s", apiURL)

	const maxRetries = 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := http.Get(apiURL)
		if err != nil {
			return nil, fmt.Errorf("HTTP请求失败: %v", err)
		}
		defer resp.Body.Close()

		// 检查响应状态码
		if resp.StatusCode == http.StatusOK {
			// 成功，解析并返回
			var repos []GitHubRepo
			if decodeErr := json.NewDecoder(resp.Body).Decode(&repos); decodeErr != nil {
				return nil, fmt.Errorf("解析JSON失败: %v", decodeErr)
			}
			logger.Printf("成功获取 %d 个公开仓库", len(repos))
			return repos, nil
		}

		// 检查速率限制错误 (403 Forbidden 或 429 Too Many Requests)
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			body, _ := ioutil.ReadAll(resp.Body)
			logger.Printf("API返回错误状态码: %d. 响应内容: %s", resp.StatusCode, string(body))

			// 尝试从响应头获取重试时间
			resetTimeStr := resp.Header.Get("X-RateLimit-Reset")
			if resetTimeStr != "" {
				// 假设 X-RateLimit-Reset 是 Unix 时间戳（秒）
				resetTime, err := time.Parse("unix", resetTimeStr)
				if err == nil {
					waitTime := time.Until(resetTime)
					if waitTime > 0 {
						logger.Printf("速率限制触发。请等待 %v 后重试...", waitTime)
						time.Sleep(waitTime + 5*time.Second) // 额外等待5秒确保冷却
						continue                             // 继续下一次循环尝试
					}
				}
			}
			// 如果无法从头获取时间，则等待一个默认时间
			waitTime := time.Duration(attempt+1) * 10 * time.Second
			logger.Printf("速率限制触发，等待 %v 后重试...", waitTime)
			time.Sleep(waitTime)
			continue
		}

		// 其他非成功状态码，直接返回错误
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("API返回错误状态码: %d，响应内容: %s", resp.StatusCode, string(body))
	}

	return nil, fmt.Errorf("达到最大重试次数 (%d) 仍无法获取仓库列表", maxRetries)
}

// 拉取或克隆仓库（并发执行）
func pullOrCloneRepos(repos []GitHubRepo) []string {
	var wg sync.WaitGroup
	// 使用 channel 来收集失败的仓库名称，避免并发写入到同一个 slice
	failedChan := make(chan string, len(repos))

	for _, repo := range repos {
		wg.Add(1)
		// 为每个仓库启动一个 Goroutine
		go func(repo GitHubRepo) {
			defer wg.Done()
			repoPath := filepath.Join(targetPath, repo.Name)

			// 检查仓库是否已存在
			exists, err := pathExists(repoPath)
			if err != nil {
				logger.Printf("检查仓库 %s 路径失败: %v", repo.Name, err)
				failedChan <- repo.Name
				return
			}

			if exists {
				logger.Printf("--- 正在处理仓库: %s (Fetching) ---", repo.Name)
				// 已存在，执行git fetch
				if err := gitFetch(repoPath); err != nil {
					logger.Printf("仓库 %s fetch失败: %v", repo.Name, err)
					failedChan <- repo.Name
				} else {
					logger.Printf("仓库 %s fetch成功", repo.Name)
				}
			} else {
				logger.Printf("--- 正在处理仓库: %s (Cloning) ---", repo.Name)
				// 不存在，执行git clone
				if err := gitClone(repo.CloneURL, repoPath); err != nil {
					logger.Printf("仓库 %s clone失败: %v", repo.Name, err)
					failedChan <- repo.Name
				} else {
					logger.Printf("仓库 %s clone成功", repo.Name)
				}
			}
		}(repo)
	}

	// 等待所有 Goroutine 完成
	wg.Wait()
	close(failedChan)

	// 收集所有失败的仓库名称
	var failedRepos []string
	for name := range failedChan {
		failedRepos = append(failedRepos, name)
	}
	return failedRepos
}

// 校验仓库是否真的拉取成功（检查目录是否存在且不为空）
func validateRepos(repos []GitHubRepo, failedRepos []string) []string {
	var validFailed []string

	for _, repoName := range failedRepos {
		repoPath := filepath.Join(targetPath, repoName)
		if exists, err := pathExists(repoPath); err != nil || !exists {
			validFailed = append(validFailed, repoName)
			continue
		}

		// 检查仓库目录是否为空（简单校验）
		files, err := ioutil.ReadDir(repoPath)
		if err != nil || len(files) == 0 {
			validFailed = append(validFailed, repoName)
		}
	}

	return validFailed
}

// 检查路径是否存在
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// 执行git clone命令
func gitClone(cloneURL string, repoPath string) error {
	cmd := exec.Command("git", "clone", cloneURL, repoPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// 执行git fetch命令（在指定仓库目录）
func gitFetch(repoPath string) error {
	cmd := exec.Command("git", "fetch")
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
