package com.aiops.agent;

import com.aiops.model.IncidentState;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.time.Instant;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Agent 抽象基类 — 模板方法模式
 *
 * 面试要点（Java 设计模式）：
 * - 模板方法模式：定义算法骨架，子类实现具体步骤
 * - 策略模式：不同 Agent 是不同的策略实现
 * - 观察者模式：通过事件总线通知其他 Agent
 */
public abstract class BaseAgent {

    protected final Logger logger = LoggerFactory.getLogger(getClass());
    private final String name;
    private final AtomicLong processedCount = new AtomicLong(0);
    private final AtomicLong errorCount = new AtomicLong(0);

    protected BaseAgent(String name) {
        this.name = name;
    }

    /**
     * 模板方法：统一的处理流程
     */
    public IncidentState handle(IncidentState state) {
        Instant start = Instant.now();
        logger.info("[{}] Processing incident {}", name, state.getIncidentId());

        try {
            state.setCurrentAgent(name);
            state.setUpdatedAt(Instant.now());

            state = process(state);

            processedCount.incrementAndGet();
            long elapsed = Duration.between(start, Instant.now()).toMillis();
            logger.info("[{}] Completed in {}ms", name, elapsed);

        } catch (Exception e) {
            errorCount.incrementAndGet();
            state.setErrorMessage(String.format("[%s] Error: %s", name, e.getMessage()));
            logger.error("[{}] Failed: {}", name, e.getMessage(), e);
        }

        return state;
    }

    protected abstract IncidentState process(IncidentState state);

    public String getName() {
        return name;
    }

    public Map<String, Object> getMetrics() {
        return Map.of(
            "agent", name,
            "processed_count", processedCount.get(),
            "error_count", errorCount.get()
        );
    }
}
