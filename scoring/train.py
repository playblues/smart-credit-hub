"""
Скрипт обучения и валидации скоринговой модели субъектов МСП.
Банковские метрики: ROC-AUC, Gini, KS-статистика, кривая калибровки.
Генерирует:
  - docs/roc_auc_curve.png   (4 графика: ROC, Калибровка, Децили, KS-кривая)
  - docs/scorecard_table.png (профессиональная таблица скоринговых баллов)
"""
from pathlib import Path
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.gridspec as gridspec
from matplotlib.patches import FancyBboxPatch
from sklearn.model_selection import StratifiedKFold, cross_val_score
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import (
    roc_auc_score, roc_curve,
    confusion_matrix, classification_report,
    brier_score_loss
)
from sklearn.calibration import calibration_curve

ALFA_RED    = "#EF3124"
ALFA_DARK   = "#1A1A2E"
ALFA_GREY   = "#8A92A6"
ALFA_GREEN  = "#28A745"
ALFA_YELLOW = "#FFC107"

def generate_if_needed():
    csv_path = Path(__file__).parent / "dataset_sme_synthetic.csv"
    if not csv_path.exists():
        from generate_dataset import generate_sme_dataset
        generate_sme_dataset()
    return csv_path

def load_features(csv_path):
    df = pd.read_csv(csv_path)
    clean = df[
        (df["bki_max_overdue_days"] < 90) &
        (df["is_liquidating"] == 0) &
        (df["business_age_months"] >= 6)
    ].copy()
    clean["business_age_years"] = clean["business_age_months"] / 12.0
    feature_cols = [
        "revenue_to_loan_ratio",
        "business_age_years",
        "bki_active_loans_count",
        "bki_had_past_delinquency",
    ]
    return clean[feature_cols], clean["default_12m"], feature_cols

# ---------------------------------------------------------------------------
# 1. Скоринговая карта (Scorecard points table)
# ---------------------------------------------------------------------------
def build_scorecard(model, feature_cols, pdo=20, target_score=500, target_odds=1.0):
    """
    Конвертирует коэффициенты логистической регрессии в скоринговые баллы
    по стандартному PDO-методу (Points to Double Odds), принятому в банках.
    """
    factor = pdo / np.log(2)
    offset = target_score - factor * np.log(target_odds)

    intercept = model.intercept_[0]
    coefs     = model.coef_[0]
    n_feats   = len(feature_cols)

    BINS = {
        "revenue_to_loan_ratio":   [0.5, 1.0, 1.5, 2.0, 2.5, 3.0],
        "business_age_years":      [0.5, 1.0, 2.0, 3.0, 5.0, 8.0],
        "bki_active_loans_count":  [0,   1,   2,   3,   4,   5  ],
        "bki_had_past_delinquency":[0,   1                       ],
    }
    LABELS = {
        "revenue_to_loan_ratio": [
            "≥ 3.0 (отл.)", "2.5–3.0", "2.0–2.5", "1.5–2.0", "1.0–1.5", "< 1.0 (плохо)"
        ],
        "business_age_years": [
            "≥ 8 лет", "5–8 лет", "3–5 лет", "2–3 года", "1–2 года", "0.5–1 год"
        ],
        "bki_active_loans_count": [
            "0 кредитов", "1 кредит", "2 кредита", "3 кредита", "4 кредита", "5+ кредитов"
        ],
        "bki_had_past_delinquency": ["Нет просрочек", "Есть просрочки"],
    }
    FEAT_NAMES = {
        "revenue_to_loan_ratio":   "Выручка / Кредит (ratio)",
        "business_age_years":      "Возраст бизнеса",
        "bki_active_loans_count":  "Кредиты в БКИ (шт.)",
        "bki_had_past_delinquency":"Просрочки в истории БКИ",
    }

    rows = []
    for i, feat in enumerate(feature_cols):
        coef = coefs[i]
        base = intercept / n_feats          # распределяем интерсепт равномерно
        bins = BINS[feat]
        labels = LABELS[feat]
        for val, label in zip(bins, labels):
            log_odds_contrib = base + coef * val
            pts = round(offset / n_feats + factor * log_odds_contrib)
            pts = max(-100, min(200, pts))  # ограничение диапазона
            rows.append({
                "feature":      FEAT_NAMES[feat],
                "label":        label,
                "value":        val,
                "points":       int(pts),
            })

    return pd.DataFrame(rows)

def plot_scorecard_table(scorecard_df, docs_dir):
    features = scorecard_df["feature"].unique()
    n_feats  = len(features)

    fig, axes = plt.subplots(1, n_feats, figsize=(18, 5.5))
    fig.patch.set_facecolor(ALFA_DARK)
    fig.suptitle(
        "Alfa SME Scoring Card  ·  Credit Risk Quantitative Model",
        color="white", fontsize=14, fontweight="bold", y=1.01
    )

    for ax, feat in zip(axes, features):
        subset = scorecard_df[scorecard_df["feature"] == feat].reset_index(drop=True)
        ax.set_facecolor("#16213E")
        ax.set_title(feat, color="white", fontsize=9, fontweight="bold", pad=8)
        ax.set_xlim(0, 1)
        ax.set_ylim(-0.5, len(subset) - 0.5)
        ax.axis("off")

        min_pts = subset["points"].min()
        max_pts = subset["points"].max()
        pts_range = max(1, max_pts - min_pts)

        for j, row in subset.iterrows():
            pts   = row["points"]
            label = row["label"]
            norm  = (pts - min_pts) / pts_range        # 0..1

            # цвет ячейки по знаку баллов
            if pts >= 40:
                bar_color = ALFA_GREEN
            elif pts >= 10:
                bar_color = ALFA_YELLOW
            else:
                bar_color = ALFA_RED

            bar_w = 0.55 * norm + 0.05
            y_pos = len(subset) - 1 - j

            ax.barh(y_pos, bar_w, height=0.7, left=0,
                    color=bar_color, alpha=0.75, zorder=2)
            ax.text(0.02, y_pos, label, va="center",
                    color="white", fontsize=7.5, zorder=3)
            sign = "+" if pts > 0 else ""
            ax.text(0.97, y_pos, f"{sign}{pts} pts",
                    va="center", ha="right",
                    color="white", fontsize=8, fontweight="bold", zorder=3)

        # горизонтальные разделители
        for j in range(len(subset)):
            ax.axhline(j - 0.5, color="#2A2A4A", lw=0.8, zorder=1)

    # легенда зон
    legend_text = (
        "Итоговые зоны:   [GREEN] >= 600 pts -> AUTO_APPROVED   "
        "[YELLOW] 500-599 pts -> COUNTER_OFFER   "
        "[GREY] 400-499 pts -> MANUAL_REVIEW   "
        "[RED] < 400 pts -> REJECTED"
    )
    fig.text(0.5, -0.03, legend_text, ha="center",
             color=ALFA_GREY, fontsize=8)

    plt.tight_layout()
    out = docs_dir / "scorecard_table.png"
    plt.savefig(out, dpi=200, bbox_inches="tight",
                facecolor=fig.get_facecolor())
    plt.close()
    return out

# ---------------------------------------------------------------------------
# 2. Основной пайплайн: метрики + 4 графика
# ---------------------------------------------------------------------------
def train_and_evaluate():
    base_dir = Path(__file__).parent
    docs_dir = base_dir.parent / "docs"
    docs_dir.mkdir(exist_ok=True)

    csv_path = generate_if_needed()
    X, y, feature_cols = load_features(csv_path)

    # --- кросс-валидация (5 фолдов) ---
    model_cv = LogisticRegression(penalty=None, solver="lbfgs",
                                  max_iter=1000, random_state=42)
    cv_scores = cross_val_score(model_cv, X, y,
                                cv=StratifiedKFold(n_splits=5, shuffle=True,
                                                   random_state=42),
                                scoring="roc_auc")

    # --- финальная модель на 75/25 ---
    from sklearn.model_selection import train_test_split
    X_tr, X_te, y_tr, y_te = train_test_split(
        X, y, test_size=0.25, random_state=42, stratify=y
    )
    model = LogisticRegression(penalty=None, solver="lbfgs",
                               max_iter=1000, random_state=42)
    model.fit(X_tr, y_tr)
    y_prob = model.predict_proba(X_te)[:, 1]
    y_pred = (y_prob >= 0.12).astype(int)

    # --- метрики ---
    auc    = roc_auc_score(y_te, y_prob)
    gini   = 2 * auc - 1
    brier  = brier_score_loss(y_te, y_prob)

    # KS-статистика
    fpr, tpr, thresholds = roc_curve(y_te, y_prob)
    ks_stat = float(np.max(tpr - fpr))
    ks_thresh_idx = int(np.argmax(tpr - fpr))

    # Lorenz / Gini curve data
    sorted_idx = np.argsort(-y_prob)
    cum_defaults = np.cumsum(y_te.values[sorted_idx]) / y_te.sum()
    cum_pop      = np.arange(1, len(y_te)+1) / len(y_te)

    # Decile default rates
    res_df = pd.DataFrame({"y_true": y_te.values, "pd": y_prob})
    res_df["decile"] = pd.qcut(res_df["pd"], q=10, labels=False) + 1
    decile_dr = res_df.groupby("decile")["y_true"].mean() * 100

    print("=" * 64)
    print("  ALFA SME CREDIT SCORECARD — VALIDATION REPORT")
    print("=" * 64)
    print(f"  Выборка (обуч / тест):   {len(X_tr)} / {len(X_te)} строк")
    print(f"  Базовый уровень дефолта: {y.mean():.2%}")
    print("-" * 64)
    ok = "[OK]"; no = "[!!]"
    print(f"  ROC-AUC (hold-out):      {auc:.4f}   (норма банка > 0.78)  {ok if auc > 0.78 else no}")
    print(f"  ROC-AUC (5-fold CV):     {cv_scores.mean():.4f} +/- {cv_scores.std():.4f}")
    print(f"  Коэф. Джини (Gini):      {gini:.4f}   (норма банка > 0.55)  {ok if gini > 0.55 else no}")
    print(f"  KS-статистика:           {ks_stat:.4f}   (норма банка > 0.35)  {ok if ks_stat > 0.35 else no}")
    print(f"  Brier Score:             {brier:.4f}   (меньше -- лучше, норма < 0.05)")
    print("-" * 64)
    print("  КОЭФФИЦИЕНТЫ МОДЕЛИ (согласованы с model.json):")
    print(f"  Intercept:               {model.intercept_[0]:+.3f}")
    for col, coef in zip(feature_cols, model.coef_[0]):
        print(f"  {col:<30s} {coef:+.3f}")
    print("=" * 64)

    # -----------------------------------------------------------------------
    # 3. Четыре графика на одном листе
    # -----------------------------------------------------------------------
    fig = plt.figure(figsize=(14, 10), facecolor=ALFA_DARK)
    gs  = gridspec.GridSpec(2, 2, hspace=0.38, wspace=0.32,
                            left=0.07, right=0.97, top=0.93, bottom=0.07)

    axes = [fig.add_subplot(gs[r, c]) for r in range(2) for c in range(2)]
    for ax in axes:
        ax.set_facecolor("#16213E")
        for spine in ax.spines.values():
            spine.set_color("#2A2A4A")
        ax.tick_params(colors=ALFA_GREY, labelsize=9)
        ax.xaxis.label.set_color(ALFA_GREY)
        ax.yaxis.label.set_color(ALFA_GREY)

    def title(ax, text):
        ax.set_title(text, color="white", fontsize=10, fontweight="bold", pad=6)

    # --- (0,0) ROC curve ---
    ax0 = axes[0]
    ax0.plot(fpr, tpr, color=ALFA_RED,  lw=2.5,
             label=f"ROC (AUC={auc:.3f}, Gini={gini:.3f})")
    ax0.plot([0,1],[0,1], color=ALFA_GREY, ls="--", lw=1.2,
             label="Random (AUC=0.50)")
    ax0.set_xlabel("False Positive Rate"); ax0.set_ylabel("True Positive Rate")
    title(ax0, "ROC-кривая")
    ax0.legend(fontsize=8, facecolor="#16213E", labelcolor="white",
               framealpha=0.9)
    ax0.grid(True, ls=":", alpha=0.4, color=ALFA_GREY)

    # --- (0,1) KS curve ---
    ax1 = axes[1]
    ax1.plot(fpr, tpr,     color=ALFA_RED,   lw=2.2, label="TPR (Sensitivity)")
    ax1.plot(fpr, fpr,     color=ALFA_GREY,  lw=1.2, ls="--")
    ax1.axvline(fpr[ks_thresh_idx], color=ALFA_YELLOW, ls=":", lw=1.5,
                label=f"KS = {ks_stat:.3f}")
    ax1.annotate(f"KS={ks_stat:.3f}", xy=(fpr[ks_thresh_idx],
                 (tpr[ks_thresh_idx]+fpr[ks_thresh_idx])/2),
                 color=ALFA_YELLOW, fontsize=9, fontweight="bold",
                 xytext=(fpr[ks_thresh_idx]+0.06,
                         (tpr[ks_thresh_idx]+fpr[ks_thresh_idx])/2))
    ax1.set_xlabel("False Positive Rate"); ax1.set_ylabel("Rate")
    title(ax1, "KS-статистика (Kolmogorov-Smirnov)")
    ax1.legend(fontsize=8, facecolor="#16213E", labelcolor="white",
               framealpha=0.9)
    ax1.grid(True, ls=":", alpha=0.4, color=ALFA_GREY)

    # --- (1,0) Calibration curve ---
    ax2 = axes[2]
    prob_true, prob_pred = calibration_curve(y_te, y_prob, n_bins=8,
                                             strategy="quantile")
    ax2.plot(prob_pred, prob_true, "o-", color=ALFA_RED,  lw=2.2, ms=6,
             label=f"Модель (Brier={brier:.4f})")
    ax2.plot([0,1],[0,1], color=ALFA_GREY, ls="--", lw=1.2,
             label="Идеальная калибровка")
    ax2.set_xlabel("Предсказанный PD"); ax2.set_ylabel("Фактическая доля дефолтов")
    title(ax2, "Кривая калибровки (Reliability Diagram)")
    ax2.legend(fontsize=8, facecolor="#16213E", labelcolor="white",
               framealpha=0.9)
    ax2.grid(True, ls=":", alpha=0.4, color=ALFA_GREY)

    # --- (1,1) Decile default rate ---
    ax3 = axes[3]
    colors_d = [ALFA_GREEN if v < 2 else ALFA_YELLOW if v < 6 else ALFA_RED
                for v in decile_dr.values]
    ax3.bar(decile_dr.index, decile_dr.values, color=colors_d,
            alpha=0.88, edgecolor="#1A1A2E", lw=0.8)
    ax3.set_xlabel("Дециль скоринга (1=лучшие, 10=худшие)")
    ax3.set_ylabel("Фактический % дефолтов")
    ax3.set_xticks(range(1, 11))
    title(ax3, "Калибровка: % дефолтов по децилям")
    ax3.grid(axis="y", ls=":", alpha=0.4, color=ALFA_GREY)

    fig.suptitle(
        f"Alfa SME Scoring Model  ·  AUC {auc:.3f}  ·  Gini {gini:.3f}  ·  KS {ks_stat:.3f}",
        color="white", fontsize=13, fontweight="bold"
    )

    out1 = docs_dir / "roc_auc_curve.png"
    plt.savefig(out1, dpi=200, facecolor=fig.get_facecolor())
    plt.close()
    print(f"  Графики сохранены: {out1}")

    # -----------------------------------------------------------------------
    # 4. Scorecard table
    # -----------------------------------------------------------------------
    sc_df = build_scorecard(model, feature_cols)
    out2  = plot_scorecard_table(sc_df, docs_dir)
    print(f"  Scorecard table:   {out2}")
    print("=" * 64)

if __name__ == "__main__":
    train_and_evaluate()
