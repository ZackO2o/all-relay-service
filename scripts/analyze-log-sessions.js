#!/usr/bin/env node

/**
 * 从日志文件分析Claude账户请求时间的CLI工具
 * 用于恢复会话窗口数据
 */

const fs = require('fs')
const path = require('path')
const readline = require('readline')
const zlib = require('zlib')
const redis = require('../src/models/redis')

class LogSessionAnalyzer {
  constructor() {
    // 更新正则表达式以匹配实际的日志格式
    this.accountUsagePattern =
      /🎯 Using sticky session shared account: (.+?) \(([a-f0-9-]{36})\) for session ([a-f0-9]+)/
    this.processingPattern =
      /📡 Processing streaming API request with usage capture for key: (.+?), account: ([a-f0-9-]{36}), session: ([a-f0-9]+)/
    this.completedPattern = /🔗 ✅ Request completed in (\d+)ms for key: (.+)/
    this.usageRecordedPattern =
      /🔗 📊 Stream usage recorded \(real\) - Model: (.+?), Input: (\d+), Output: (\d+), Cache Create: (\d+), Cache Read: (\d+), Total: (\d+) tokens/
    this.timestampPattern = /\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]/
    this.accounts = new Map()
    this.requestHistory = []
    this.sessions = new Map() // 记录会话信息
  }

  // 解析时间戳
  parseTimestamp(line) {
    const match = line.match(this.timestampPattern)
    if (match) {
      return new Date(match[1])
    }
    return null
  }

  // 分析单个日志文件
  async analyzeLogFile(filePath) {
    console.log(`📖 分析日志文件: ${filePath}`)

    let fileStream = fs.createReadStream(filePath)

    // 如果是gz文件，需要先解压
    if (filePath.endsWith('.gz')) {
      console.log('   🗜️  检测到gz压缩文件，正在解压...')
      fileStream = fileStream.pipe(zlib.createGunzip())
    }

    const rl = readline.createInterface({
      input: fileStream,
      crlfDelay: Infinity
    })

    let lineCount = 0
    let requestCount = 0
    let usageCount = 0

    for await (const line of rl) {
      lineCount++

      // 解析时间戳
      const timestamp = this.parseTimestamp(line)
      if (!timestamp) {
        continue
      }

      // 查找账户使用记录
      const accountUsageMatch = line.match(this.accountUsagePattern)
      if (accountUsageMatch) {
        const accountName = accountUsageMatch[1]
        const accountId = accountUsageMatch[2]
        const sessionId = accountUsageMatch[3]

        if (!this.accounts.has(accountId)) {
          this.accounts.set(accountId, {
            accountId,
            accountName,
            requests: [],
            firstRequest: timestamp,
            lastRequest: timestamp,
            totalRequests: 0,
            sessions: new Set()
          })
        }

        const account = this.accounts.get(accountId)
        account.sessions.add(sessionId)

        if (timestamp < account.firstRequest) {
          account.firstRequest = timestamp
        }
        if (timestamp > account.lastRequest) {
          account.lastRequest = timestamp
        }
      }

      // 查找请求处理记录
      const processingMatch = line.match(this.processingPattern)
      if (processingMatch) {
        const apiKeyName = processingMatch[1]
        const accountId = processingMatch[2]
        const sessionId = processingMatch[3]

        if (!this.accounts.has(accountId)) {
          this.accounts.set(accountId, {
            accountId,
            accountName: 'Unknown',
            requests: [],
            firstRequest: timestamp,
            lastRequest: timestamp,
            totalRequests: 0,
            sessions: new Set()
          })
        }

        const account = this.accounts.get(accountId)
        account.requests.push({
          timestamp,
          apiKeyName,
          sessionId,
          type: 'processing'
        })

        account.sessions.add(sessionId)
        account.totalRequests++
        requestCount++

        if (timestamp > account.lastRequest) {
          account.lastRequest = timestamp
        }

        // 记录到全局请求历史
        this.requestHistory.push({
          timestamp,
          accountId,
          apiKeyName,
          sessionId,
          type: 'processing'
        })
      }

      // 查找请求完成记录
      const completedMatch = line.match(this.completedPattern)
      if (completedMatch) {
        const duration = parseInt(completedMatch[1])
        const apiKeyName = completedMatch[2]

        // 记录到全局请求历史
        this.requestHistory.push({
          timestamp,
          apiKeyName,
          duration,
          type: 'completed'
        })
      }

      // 查找使用统计记录
      const usageMatch = line.match(this.usageRecordedPattern)
      if (usageMatch) {
        const model = usageMatch[1]
        const inputTokens = parseInt(usageMatch[2])
        const outputTokens = parseInt(usageMatch[3])
        const cacheCreateTokens = parseInt(usageMatch[4])
        const cacheReadTokens = parseInt(usageMatch[5])
        const totalTokens = parseInt(usageMatch[6])

        usageCount++

        // 记录到全局请求历史
        this.requestHistory.push({
          timestamp,
          type: 'usage',
          model,
          inputTokens,
          outputTokens,
          cacheCreateTokens,
          cacheReadTokens,
          totalTokens
        })
      }
    }

    console.log(
      `   📊 解析完成: ${lineCount} 行, 找到 ${requestCount} 个请求记录, ${usageCount} 个使用统计`
    )
  }

  // 分析日志目录中的所有文件
  async analyzeLogDirectory(logDir = './logs') {
    console.log(`🔍 扫描日志目录: ${logDir}\n`)

    try {
      const files = fs.readdirSync(logDir)
      const logFiles = files
        .filter(
          (file) =>
            file.includes('all-relay') &&
            (file.endsWith('.log') ||
              file.endsWith('.log.1') ||
              file.endsWith('.log.gz') ||
              file.match(/\.log\.\d+\.gz$/) ||
              file.match(/\.log\.\d+$/))
        )
        .sort()
        .reverse() // 最新的文件优先

      if (logFiles.length === 0) {
        console.log('❌ 没有找到日志文件')
        return
      }

      console.log(`📁 找到 ${logFiles.length} 个日志文件:`)
      logFiles.forEach((file) => console.log(`   - ${file}`))
      console.log('')

      // 分析每个文件
      for (const file of logFiles) {
        const filePath = path.join(logDir, file)
        await this.analyzeLogFile(filePath)
      }
    } catch (error) {
      console.error(`❌ 读取日志目录失败: ${error.message}`)
      throw error
    }
  }

  // 分析单个日志文件（支持直接传入文件路径）
  async analyzeSingleFile(filePath) {
    console.log(`🔍 分析单个日志文件: ${filePath}\n`)

    try {
      if (!fs.existsSync(filePath)) {
        console.log('❌ 文件不存在')
        return
      }

      await this.analyzeLogFile(filePath)
    } catch (error) {
      console.error(`❌ 分析文件失败: ${error.message}`)
      throw error
    }
  }

  // 计算会话窗口
  calculateSessionWindow(requestTime) {
    const hour = requestTime.getHours()
    const windowStartHour = Math.floor(hour / 5) * 5

    const windowStart = new Date(requestTime)
    windowStart.setHours(windowStartHour, 0, 0, 0)

    const windowEnd = new Date(windowStart)
    windowEnd.setHours(windowEnd.getHours() + 5)

    return { windowStart, windowEnd }
  }

  // 分析会话窗口
  analyzeSessionWindows() {
    console.log('🕐 分析会话窗口...\n')

    const now = new Date()
    const results = []

    for (const [accountId, accountData] of this.accounts) {
      const requests = accountData.requests.sort((a, b) => a.timestamp - b.timestamp)

      // 按会话窗口分组请求
      const windowGroups = new Map()

      for (const request of requests) {
        const { windowStart, windowEnd } = this.calculateSessionWindow(request.timestamp)
        const windowKey = `${windowStart.getTime()}-${windowEnd.getTime()}`

        if (!windowGroups.has(windowKey)) {
          windowGroups.set(windowKey, {
            windowStart,
            windowEnd,
            requests: [],
            isActive: now >= windowStart && now < windowEnd
          })
        }

        windowGroups.get(windowKey).requests.push(request)
      }

      // 转换为数组并排序
      const windowArray = Array.from(windowGroups.values()).sort(
        (a, b) => b.windowStart - a.windowStart
      ) // 最新的窗口优先

      const result = {
        accountId,
        accountName: accountData.accountName,
        totalRequests: accountData.totalRequests,
        firstRequest: accountData.firstRequest,
        lastRequest: accountData.lastRequest,
        sessions: accountData.sessions,
        windows: windowArray,
        currentActiveWindow: windowArray.find((w) => w.isActive) || null,
        mostRecentWindow: windowArray[0] || null
      }

      results.push(result)
    }

    return results.sort((a, b) => b.lastRequest - a.lastRequest)
  }

  // 显示分析结果
  displayResults(results) {
    console.log('📊 分析结果:\n')
    console.log('='.repeat(80))

    for (const result of results) {
      console.log(`🏢 账户: ${result.accountName || 'Unknown'} (${result.accountId})`)
      console.log(`   总请求数: ${result.totalRequests}`)
      console.log(`   会话数: ${result.sessions ? result.sessions.size : 0}`)
      console.log(`   首次请求: ${result.firstRequest.toLocaleString()}`)
      console.log(`   最后请求: ${result.lastRequest.toLocaleString()}`)

      if (result.currentActiveWindow) {
        console.log(
          `   ✅ 当前活跃窗口: ${result.currentActiveWindow.windowStart.toLocaleString()} - ${result.currentActiveWindow.windowEnd.toLocaleString()}`
        )
        console.log(`       窗口内请求: ${result.currentActiveWindow.requests.length} 次`)
        const progress = this.calculateWindowProgress(
          result.currentActiveWindow.windowStart,
          result.currentActiveWindow.windowEnd
        )
        console.log(`       窗口进度: ${progress}%`)
      } else if (result.mostRecentWindow) {
        const window = result.mostRecentWindow
        console.log(
          `   ⏰ 最近窗口(已过期): ${window.windowStart.toLocaleString()} - ${window.windowEnd.toLocaleString()}`
        )
        console.log(`       窗口内请求: ${window.requests.length} 次`)
        const hoursAgo = Math.round((new Date() - window.windowEnd) / (1000 * 60 * 60))
        console.log(`       过期时间: ${hoursAgo} 小时前`)
      } else {
        console.log('   ❌ 无会话窗口数据')
      }

      // 显示最近几个窗口
      if (result.windows.length > 1) {
        console.log(`   📈 历史窗口: ${result.windows.length} 个`)
        const recentWindows = result.windows.slice(0, 3)
        for (let i = 0; i < recentWindows.length; i++) {
          const window = recentWindows[i]
          const status = window.isActive ? '活跃' : '已过期'
          console.log(
            `      ${i + 1}. ${window.windowStart.toLocaleString()} - ${window.windowEnd.toLocaleString()} (${status}, ${window.requests.length}次请求)`
          )
        }
      }

      // 显示最近几个会话的API Key使用情况
      const accountData = this.accounts.get(result.accountId)
      if (accountData && accountData.requests && accountData.requests.length > 0) {
        const apiKeyStats = {}

        for (const req of accountData.requests) {
          if (!apiKeyStats[req.apiKeyName]) {
            apiKeyStats[req.apiKeyName] = 0
          }
          apiKeyStats[req.apiKeyName]++
        }

        console.log('   🔑 API Key使用统计:')
        for (const [keyName, count] of Object.entries(apiKeyStats)) {
          console.log(`      - ${keyName}: ${count} 次`)
        }
      }

      console.log('')
    }

    console.log('='.repeat(80))
    console.log(`总计: ${results.length} 个账户, ${this.requestHistory.length} 个日志记录\n`)
  }

  // 计算窗口进度百分比
  calculateWindowProgress(windowStart, windowEnd) {
    const now = new Date()
    const totalDuration = windowEnd.getTime() - windowStart.getTime()
    const elapsedTime = now.getTime() - windowStart.getTime()
    return Math.max(0, Math.min(100, Math.round((elapsedTime / totalDuration) * 100)))
  }

  // 更新Redis中的会话窗口数据
  async updateRedisSessionWindows(results, dryRun = true) {
    if (dryRun) {
      console.log('🧪 模拟模式 - 不会实际更新Redis数据\n')
    } else {
      console.log('💾 更新Redis中的会话窗口数据...\n')
      await redis.connect()
    }

    let updatedCount = 0
    let skippedCount = 0

    for (const result of results) {
      try {
        const accountData = await redis.getClaudeAccount(result.accountId)

        if (!accountData || Object.keys(accountData).length === 0) {
          console.log(`⚠️  账户 ${result.accountId} 在Redis中不存在，跳过`)
          skippedCount++
          continue
        }

        console.log(`🔄 处理账户: ${accountData.name || result.accountId}`)

        // 确定要设置的会话窗口
        let targetWindow = null

        if (result.currentActiveWindow) {
          targetWindow = result.currentActiveWindow
          console.log(
            `   ✅ 使用当前活跃窗口: ${targetWindow.windowStart.toLocaleString()} - ${targetWindow.windowEnd.toLocaleString()}`
          )
        } else if (result.mostRecentWindow) {
          const window = result.mostRecentWindow
          const now = new Date()

          // 如果最近窗口是在过去24小时内的，可以考虑恢复
          const hoursSinceWindow = (now - window.windowEnd) / (1000 * 60 * 60)

          if (hoursSinceWindow <= 24) {
            console.log(
              `   🕐 最近窗口在24小时内，但已过期: ${window.windowStart.toLocaleString()} - ${window.windowEnd.toLocaleString()}`
            )
            console.log(`   ❌ 不恢复已过期窗口（${hoursSinceWindow.toFixed(1)}小时前过期）`)
          } else {
            console.log('   ⏰ 最近窗口超过24小时前，不予恢复')
          }
        }

        if (targetWindow && !dryRun) {
          // 更新Redis中的会话窗口数据
          accountData.sessionWindowStart = targetWindow.windowStart.toISOString()
          accountData.sessionWindowEnd = targetWindow.windowEnd.toISOString()
          accountData.lastUsedAt = result.lastRequest.toISOString()
          accountData.lastRequestTime = result.lastRequest.toISOString()

          await redis.setClaudeAccount(result.accountId, accountData)
          updatedCount++

          console.log('   ✅ 已更新会话窗口数据')
        } else if (targetWindow) {
          updatedCount++
          console.log(
            `   🧪 [模拟] 将设置会话窗口: ${targetWindow.windowStart.toLocaleString()} - ${targetWindow.windowEnd.toLocaleString()}`
          )
        } else {
          skippedCount++
          console.log('   ⏭️  跳过（无有效窗口）')
        }

        console.log('')
      } catch (error) {
        console.error(`❌ 处理账户 ${result.accountId} 时出错: ${error.message}`)
        skippedCount++
      }
    }

    if (!dryRun) {
      await redis.disconnect()
    }

    console.log('📊 更新结果:')
    console.log(`   ✅ 已更新: ${updatedCount}`)
    console.log(`   ⏭️  已跳过: ${skippedCount}`)
    console.log(`   📋 总计: ${results.length}`)
  }

  // 主分析函数
  async analyze(options = {}) {
    const { logDir = './logs', singleFile = null, updateRedis = false, dryRun = true } = options

    try {
      console.log('🔍 Claude账户会话窗口分析工具\n')

      // 分析日志文件
      if (singleFile) {
        await this.analyzeSingleFile(singleFile)
      } else {
        await this.analyzeLogDirectory(logDir)
      }

      if (this.accounts.size === 0) {
        console.log('❌ 没有找到任何Claude账户的请求记录')
        return []
      }

      // 分析会话窗口
      const results = this.analyzeSessionWindows()

      // 显示结果
      this.displayResults(results)

      // 更新Redis（如果需要）
      if (updateRedis) {
        await this.updateRedisSessionWindows(results, dryRun)
      }

      return results
    } catch (error) {
      console.error('❌ 分析失败:', error)
      throw error
    }
  }
}

// 命令行参数解析
function parseArgs() {
  const args = process.argv.slice(2)
  const options = {
    logDir: './logs',
    singleFile: null,
    updateRedis: false,
    dryRun: true
  }

  for (const arg of args) {
    if (arg.startsWith('--log-dir=')) {
      options.logDir = arg.split('=')[1]
    } else if (arg.startsWith('--file=')) {
      options.singleFile = arg.split('=')[1]
    } else if (arg === '--update-redis') {
      options.updateRedis = true
    } else if (arg === '--no-dry-run') {
      options.dryRun = false
    } else if (arg === '--help' || arg === '-h') {
      showHelp()
      process.exit(0)
    }
  }

  return options
}

// 显示帮助信息
function showHelp() {
  console.log(`
Claude账户会话窗口日志分析工具

从日志文件中分析Claude账户的请求时间，计算会话窗口，并可选择性地更新Redis数据。

用法:
  node scripts/analyze-log-sessions.js [选项]

选项:
  --log-dir=PATH       日志文件目录 (默认: ./logs)
  --file=PATH          分析单个日志文件
  --update-redis       更新Redis中的会话窗口数据
  --no-dry-run         实际执行Redis更新（默认为模拟模式）
  --help, -h           显示此帮助信息

示例:
  # 分析默认日志目录
  node scripts/analyze-log-sessions.js

  # 分析指定目录的日志
  node scripts/analyze-log-sessions.js --log-dir=/path/to/logs

  # 分析单个日志文件
  node scripts/analyze-log-sessions.js --file=/path/to/logfile.log

  # 模拟更新Redis数据（不实际更新）
  node scripts/analyze-log-sessions.js --file=/path/to/logfile.log --update-redis

  # 实际更新Redis数据
  node scripts/analyze-log-sessions.js --file=/path/to/logfile.log --update-redis --no-dry-run

会话窗口规则:
  - Claude官方规定每5小时为一个会话窗口
  - 窗口按整点对齐（如 05:00-10:00, 10:00-15:00）
  - 只有当前时间在窗口内的才被认为是活跃窗口
  - 工具会自动识别并恢复活跃的会话窗口
`)
}

// 主函数
async function main() {
  try {
    const options = parseArgs()

    const analyzer = new LogSessionAnalyzer()
    await analyzer.analyze(options)

    console.log('🎉 分析完成')
  } catch (error) {
    console.error('💥 程序执行失败:', error)
    process.exit(1)
  }
}

// 如果直接运行此脚本
if (require.main === module) {
  main()
}

module.exports = LogSessionAnalyzer
