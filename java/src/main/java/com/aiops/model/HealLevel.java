package com.aiops.model;

/**
 * 自愈分级策略
 * L0: 全自动执行，无需人工确认
 * L1: 半自动，需要 oncall 确认
 * L2: 人工介入，需要 TL 审批
 */
public enum HealLevel {
    L0_AUTO, L1_SEMI, L2_MANUAL
}
