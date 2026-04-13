# 🚀 GitHub Repository Bulk Download Tool (github-user-dl)

This project is a command-line tool written in Go for automatically downloading all public repositories from a specified GitHub user to a local directory.

## ✨ Key Features & Optimizations

This optimized version significantly enhances robustness and performance:

1.  **🚀 Performance Boost (Concurrency):** Repository pull/clone operations are now **concurrently executed**, greatly speeding up processing of large numbers of repositories.
2.  **🛡️ Robustness Enhancement (API Rate Limit Handling):** Automatically detects GitHub API rate limit errors (403/429). When limits are triggered, the program reads the reset time from response headers and performs exponential backoff retries until success or max retries reached.
3.  **✅ Process Optimization:** Implements a complete retry loop until all repositories are successfully pulled or max retries reached.
4.  **🧹 Code Quality:** Optimized internal functions like path checking and logging to follow Go best practices.

## ⚙️ Usage

### 1. Install Dependencies

Ensure you have Go environment installed and the `git` command-line tool available.

### 2. Run the Program

Two ways to run:

**A. Development/Debug Mode (Recommended)**
Use `go run` for quick iteration:
```bash
# Syntax: go run main.go -u <username> [-p <target_path>]

# Example 1: Download all repos from 'openclaw' to default directory (./openclaw)
go run main.go -u openclaw

# Example 2: Download all repos from 'octocat' to custom directory
go run main.go -u octocat -p ./my_github_repos
```

**B. Production/Deployment Mode (Recommended)**
Compile to binary first, then run:
```bash
# 1. Compile
go build -o gh-user-dl ./main.go

# 2. Run the binary
./gh-user-dl -u <username> [-p <target_path>]
```

### 3. Parameters

*   `-u <username>`: **(Required)** GitHub username to download repositories from.
*   `-p <target_path>`: **(Optional)** Local root directory for all repositories. Default: `./<username>`.

## 🔄 Workflow

1.  **Initialize:** Program creates the target root directory (e.g., `./openclaw`).
2.  **Fetch List:** Calls GitHub API to get all public repositories. Includes rate limit retry mechanism.
3.  **Concurrent Processing:** Iterates through all repositories using Goroutines:
    *   **Check:** Checks if local directory already exists.
    *   **Fetch (Exists):** If exists, runs `git fetch` to update.
    *   **Clone (Not Exists):** If not exists, runs `git clone` to download.
4.  **Validation & Retry:** Collects failed repositories, waits, then repeats steps 2-3 until all synced.

## ⚠️ Rate Limits & Authentication

*   **Unauthenticated Users:** Limited to 60 requests/hour.
*   **Recommended:** Set **GitHub Personal Access Token (PAT)** environment variable for higher limits:
    ```bash
    export GITHUB_TOKEN="YOUR_PAT_HERE"
    # Then modify main.go to read this token and add to HTTP request headers
    ```

## 📄 Logging

All execution details, success/failure logs, API errors, and retry info are recorded in `github-dl.log` under the target directory.

## 🌍 Internationalization

This tool supports **English** and **Chinese** languages. The default language follows your system settings:

- **English:** When system language is English (e.g., `LANG=en_US.UTF-8`)
- **Chinese:** When system language is Chinese (e.g., `LANG=zh_CN.UTF-8`)

You can also force a specific language:
```bash
LANG=en_US.UTF-8 ./gh-user-dl -u username  # English
LANG=zh_CN.UTF-8 ./gh-user-dl -u username  # Chinese
```