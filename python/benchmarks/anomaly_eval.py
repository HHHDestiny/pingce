"""异常检测评测脚本 —— 给简历里的"误报率降低 85% / 准确率 92%"找真实依据

运行：
    cd python && python benchmarks/anomaly_eval.py

输出三种策略在同一份标注数据集上的 Precision / Recall / FPR，
并算出"三算法投票 相对 静态阈值"的误报率下降幅度。

依赖：numpy（必需）、scikit-learn（可选，缺失时自动降级为双算法投票）
"""

import sys
import os

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import numpy as np
from models.time_series import (
    ThreeSigmaDetector,
    EWMADetector,
    IsolationForestDetector,
)


def generate_labeled_series(
    n_points: int = 600,
    anomaly_ratio: float = 0.03,
    seed: int = 42,
):
    """生成带标注的时序数据

    构成：线性趋势 + 正弦周期 + 高斯噪声 + 随机注入的尖刺/跌落异常
    返回 (values, is_anomaly_labels)
    """
    rng = np.random.default_rng(seed)
    t = np.arange(n_points)

    baseline = 50 + 0.05 * t                        # 缓慢上涨的趋势
    seasonal = 10 * np.sin(2 * np.pi * t / 24)      # 日周期波动
    noise = rng.normal(0, 2, n_points)              # 噪声

    values = baseline + seasonal + noise
    labels = np.zeros(n_points, dtype=int)

    n_anomalies = int(n_points * anomaly_ratio)
    # 避开前 60 个点，给滑窗留足历史
    indices = rng.choice(np.arange(60, n_points), size=n_anomalies, replace=False)
    for idx in indices:
        values[idx] += rng.choice([-1, 1]) * rng.uniform(20, 35)
        labels[idx] = 1

    return values.tolist(), labels.tolist()


def evaluate(values, labels, strategy, warmup=50, window=120):
    """滑窗评测某个检测策略

    window：基线窗口大小。默认只取最近 120 个点做基线（滚动基线），
    这是工程上的常规做法——用全量历史会把趋势和季节性算进 std，
    导致阈值被拉宽、异常漏检。
    """
    tp = fp = fn = tn = 0

    for i in range(warmup, len(values)):
        history = values[:i] if window is None else values[max(0, i - window):i]
        current = values[i]
        truth = labels[i] == 1

        predicted = strategy(history, current, i)

        if predicted and truth:
            tp += 1
        elif predicted and not truth:
            fp += 1
        elif not predicted and truth:
            fn += 1
        else:
            tn += 1

    precision = tp / (tp + fp) if (tp + fp) else 0.0
    recall = tp / (tp + fn) if (tp + fn) else 0.0
    fpr = fp / (fp + tn) if (fp + tn) else 0.0
    f1 = 2 * precision * recall / (precision + recall) if (precision + recall) else 0.0

    return {
        "tp": tp, "fp": fp, "fn": fn, "tn": tn,
        "precision": round(precision, 4),
        "recall": round(recall, 4),
        "fpr": round(fpr, 4),
        "f1": round(f1, 4),
    }


def main():
    try:
        from sklearn.ensemble import IsolationForest  # noqa: F401
        has_sklearn = True
    except ImportError:
        has_sklearn = False
        print("[warn] scikit-learn 未安装，Isolation Forest 将弃权，退化为双算法投票")

    values, labels = generate_labeled_series(n_points=600, anomaly_ratio=0.03)
    n_anomalies = sum(labels)
    print(f"\n数据集：600 个点，注入异常 {n_anomalies} 个")
    print("=" * 74)

    three_sigma = ThreeSigmaDetector(sigma_multiplier=3.0)
    ewma = EWMADetector(alpha=0.3, threshold=3.0)
    iso = IsolationForestDetector(contamination=0.05)

    def static_threshold(history, current, i):
        """对照组：静态阈值 CPU > 80%"""
        return current > 80.0

    def only_3sigma(history, current, i):
        return three_sigma.detect(history, current).is_anomaly

    def only_ewma(history, current, i):
        return ewma.detect(history, current).is_anomaly

    def yoy_3sigma(history, current, i, period=24):
        """同比基线 3-Sigma：只和历史上同一相位（同一时刻）的数据比

        这是工业界的常规做法——先剥离周期性和趋势，再对残差做 3-Sigma，
        避免把大促高峰这类周期性波动误判为异常。
        """
        start = i - len(history)
        same_phase = [
            history[r] for r in range(len(history))
            if (start + r) % period == i % period
        ]
        if len(same_phase) < 5:
            return False
        mu = float(np.median(same_phase))
        sd = float(np.std(same_phase))
        if sd == 0:
            return False
        return abs(current - mu) / sd > 3.0

    def ensemble(history, current, i):
        """三算法投票，至少 2 票（IF 样本不足时弃权）"""
        votes = 0
        if three_sigma.detect(history, current).is_anomaly:
            votes += 1
        if ewma.detect(history, current).is_anomaly:
            votes += 1
        if has_sklearn and len(history) >= 20:
            if iso.detect(history, current).is_anomaly:
                votes += 1
        return votes >= 2

    def ensemble_yoy(history, current, i):
        """改进版投票：用同比 3-Sigma 替换原始 3-Sigma，再和 EWMA 投票"""
        votes = 0
        if yoy_3sigma(history, current, i):
            votes += 1
        if ewma.detect(history, current).is_anomaly:
            votes += 1
        if has_sklearn and len(history) >= 20:
            if iso.detect(history, current).is_anomaly:
                votes += 1
        return votes >= 2

    strategies = [
        ("静态阈值 (>80)", static_threshold, 120),
        ("单算法 3-Sigma", only_3sigma, 120),
        ("单算法 EWMA", only_ewma, 120),
        ("三算法投票 (>=2)", ensemble, 120),
        ("同比3σ + EWMA投票", ensemble_yoy, 336),
        ("投票·全量基线", ensemble, None),
    ]

    results = {}
    header = f"{'策略':<20}{'TP':>6}{'FP':>6}{'FN':>6}{'精确率':>10}{'召回率':>10}{'误报率':>10}{'F1':>9}"
    print(header)
    print("-" * 74)

    for name, fn, win in strategies:
        r = evaluate(values, labels, fn, window=win)
        results[name] = r
        print(
            f"{name:<20}{r['tp']:>6}{r['fp']:>6}{r['fn']:>6}"
            f"{r['precision']:>10.2%}{r['recall']:>10.2%}{r['fpr']:>10.2%}{r['f1']:>9.3f}"
        )

    print("=" * 74)
    print("注：前 4 行用滚动窗口基线（最近 120 点），第 5 行用全量历史基线做对照。")

    base = results["静态阈值 (>80)"]
    best = results["三算法投票 (>=2)"]

    if base["fpr"] > 0:
        reduction = (base["fpr"] - best["fpr"]) / base["fpr"]
        print(f"\n误报率：  {base['fpr']:.2%}  →  {best['fpr']:.2%}   相对下降 {reduction:.1%}")
    else:
        print("\n对照组误报率为 0，可尝试调低静态阈值或增大异常注入强度再测")

    print(f"精确率：  {base['precision']:.2%}  →  {best['precision']:.2%}")
    print(f"召回率：  {base['recall']:.2%}  →  {best['recall']:.2%}")
    print(f"F1：      {base['f1']:.3f}  →  {best['f1']:.3f}")
    if '同比3σ + EWMA投票' in results:
        y = results['同比3σ + EWMA投票']
        print(f"\n[改进版] 同比3σ + EWMA 投票：精确率 {y['precision']:.2%} / 召回率 {y['recall']:.2%} / F1 {y['f1']:.3f}")

    print("\n>> 把上面两个真实数字填进简历，替换掉 85% / 92% 这两个未实测的估值。")
    print("   口径说明：精确率 = 报出的告警中真异常的比例；误报率 = 正常点被误报的比例。")


if __name__ == "__main__":
    main()
