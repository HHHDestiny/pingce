# Multi-Agent AIOps — 多 Agent 智能运维系统

> 企业级多 Agent 智能运维系统，支持 Python / Java / Go 三种语言实现。
> 面向求职者的完整项目：代码 + 架构文档 + 面试八股文 + 简历模板 + STAR 话术。

## 项目简介

本项目实现了一个基于**事件驱动架构**的多 Agent 智能运维平台（AIOps），包含四个核心 Agent：

| Agent | 职责 | 技术亮点 |
|-------|------|----------|
| 监控告警 Agent | 时序异常检测、告警收敛去噪、告警分级 | Prophet + Isolation Forest 动态阈值 |
| 根因分析 Agent | 知识图谱遍历、因果推断、多源关联 | Neo4j 图遍历 + 贝叶斯推理 |
| 故障自愈 Agent | 修复方案匹配、自动执行、回滚验证 | 分级策略 L0/L1/L2 + 安全护栏 |
| 变更审批 Agent | 风险评估、爆炸半径分析、审批门控 | RBAC + 审计日志 + 自动评分 |

## 架构总览

```
┌─────────────────────────────────────────────────────────┐
│                    数据采集层                              │
│  Prometheus │ Loki │ Jaeger │ CMDB                       │
└──────────────────────┬──────────────────────────────────┘
                       │
              ┌────────▼────────┐
              │   事件总线 Kafka  │
              └────────┬────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│                  Agent 编排器 (Orchestrator)               │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ 监控告警  │→│ 根因分析  │→│ 故障自愈  │→│ 变更审批  │   │
│  │  Agent   │ │  Agent   │ │  Agent   │ │  Agent   │   │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│                     知识层                                │
│  Neo4j 知识图谱 │ 向量数据库 RAG │ 规则引擎               │
└─────────────────────────────────────────────────────────┘
```

## 快速开始

### Python 版（推荐）

```bash
cd python
pip install -r requirements.txt
# 启动基础设施（Kafka + Neo4j + Prometheus）
docker-compose up -d
# 运行主服务
python -m uvicorn api.main:app --reload --port 8000
```

### Java 版

```bash
cd java
mvn clean package
docker-compose up -d
java -jar target/aiops-agent-1.0.0.jar
```

### Go 版

```bash
cd golang
go mod tidy
docker-compose up -d
go run cmd/server/main.go
```

## 项目结构

```
multi-agent-aiops/
├── python/          # Python 实现 (LangGraph + FastAPI)
├── java/            # Java 实现 (Spring Boot + Spring AI)
├── golang/          # Go 实现 (Gin + goroutine/channel)
├── docs/
│   ├── architecture.md        # 架构设计文档
│   ├── interview/             # 面试准备材料
│   │   ├── resume-template.md # 简历模板
│   │   ├── star-method.md     # STAR 法则话术
│   │   ├── interview-qa.md    # 面试 Q&A 100 问
│   │   └── baguwen.md         # 八股文
│   └── tutorial/              # 从 0 到部署教程
│       ├── 01-getting-started.md
│       ├── 02-event-driven.md
│       ├── 03-agent-design.md
│       ├── 04-knowledge-graph.md
│       └── 05-deploy.md
└── README.md
```

## 技术栈

| 层次 | Python 版 | Java 版 | Go 版 |
|------|-----------|---------|-------|
| Agent 框架 | LangGraph | Spring AI + 自建编排器 | goroutine + channel |
| API | FastAPI | Spring Boot 3 | Gin |
| 事件总线 | confluent-kafka | Spring Cloud Stream | sarama |
| 知识图谱 | Neo4j + py2neo | Spring Data Neo4j | neo4j-go-driver |
| 时序分析 | Prophet + sklearn | Commons Math + EWMA | gonum |
| 向量数据库 | ChromaDB | -- | -- |
| 容器化 | Docker Compose | Docker Compose | Docker Compose |

## 面试亮点

1. **事件驱动架构**：Kafka 解耦 Agent 间通信，支持背压和重试
2. **时序异常检测**：动态阈值替代静态阈值，提前 N 分钟预警
3. **知识图谱**：Neo4j 存储服务拓扑 + 依赖关系，图遍历做根因推理
4. **分级自愈**：L0 全自动 / L1 半自动 / L2 人工，配合安全护栏
5. **生产级设计**：熔断器、限流、dry-run、审计日志、RBAC

## 适用人群

- 准备大厂面试（腾讯/美团/字节/阿里等 AIOps 方向）的求职者
- 想要系统学习多 Agent 协作架构的开发者
- 对 AIOps 智能运维感兴趣的运维/SRE 工程师

## License

MIT License
