"""
Скрипт обучения и валидации скоринговой модели субъектов МСП.
Рассчитывает банковские валидационные метрики: ROC-AUC, Gini, Confusion Matrix.
Сохраняет график ROC-кривой в docs/roc_auc_curve.png.
"""
from pathlib import Path
import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
from sklearn.model_selection import train_test_split
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import roc_auc_score, roc_curve, confusion_matrix, classification_report

def train_and_evaluate():
    base_dir = Path(__file__).parent
    docs_dir = base_dir.parent / "docs"
    docs_dir.mkdir(exist_ok=True)
    
    csv_path = base_dir / "dataset_sme_synthetic.csv"
    if not csv_path.exists():
        from generate_dataset import generate_sme_dataset
        generate_sme_dataset()
        
    df = pd.read_csv(csv_path)
    
    # Исключаем стоп-факторы для скоринговой модели (они отсекаются правилами до скоринга)
    clean_df = df[
        (df["bki_max_overdue_days"] < 90) &
        (df["is_liquidating"] == 0) &
        (df["business_age_months"] >= 6)
    ].copy()
    
    clean_df["business_age_years"] = clean_df["business_age_months"] / 12.0
    
    feature_cols = [
        "revenue_to_loan_ratio",
        "business_age_years",
        "bki_active_loans_count",
        "bki_had_past_delinquency"
    ]
    
    X = clean_df[feature_cols]
    y = clean_df["default_12m"]
    
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.25, random_state=42, stratify=y
    )
    
    model = LogisticRegression(penalty=None, solver="lbfgs", random_state=42)
    model.fit(X_train, y_train)
    
    # Предсказание вероятностей (PD)
    y_pred_proba = model.predict_proba(X_test)[:, 1]
    y_pred = (y_pred_proba >= 0.12).astype(int)  # порог отсечения желтой/серой зоны
    
    # Метрики
    auc = roc_auc_score(y_test, y_pred_proba)
    gini = 2 * auc - 1
    cm = confusion_matrix(y_test, y_pred)
    
    print("=" * 60)
    print("РЕЗУЛЬТАТЫ ОБУЧЕНИЯ СКОРИНГОВОЙ МОДЕЛИ (ALFA SME)")
    print("=" * 60)
    print(f"Размер обучающей выборки: {len(X_train)} строк")
    print(f"Размер тестовой выборки:  {len(X_test)} строк")
    print(f"Базовый уровень дефолта:  {y.mean():.2%}")
    print("-" * 60)
    print(f"ROC-AUC:                  {auc:.4f}  (требование банка: > 0.78)")
    print(f"Коэффициент Джини (Gini): {gini:.4f}  (требование банка: > 0.55)")
    print("-" * 60)
    print("Матрица ошибок (Confusion Matrix, порог 12%):")
    print(cm)
    print("\nКлассификационный отчет:")
    print(classification_report(y_test, y_pred, target_names=["Good (0)", "Default (1)"]))
    
    print("=" * 60)
    print("ПОЛУЧЕННЫЕ КОЭФФИЦИЕНТЫ МОДЕЛИ (согласованы с model.json):")
    print(f"Intercept (w0): {model.intercept_[0]:.3f}")
    for col, coef in zip(feature_cols, model.coef_[0]):
        print(f"  {col:25s}: {coef:+.3f}")
    print("=" * 60)
    
    # Построение графика ROC-кривой и распределения скоринга
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(13, 5))
    
    # 1. ROC Curve
    fpr, tpr, _ = roc_curve(y_test, y_pred_proba)
    ax1.plot(fpr, tpr, color="#EF3124", lw=2.5, label=f"ROC кривая (AUC = {auc:.3f}, Gini = {gini:.3f})")
    ax1.plot([0, 1], [0, 1], color="#8A92A6", linestyle="--", lw=1.5, label="Случайный выбор (AUC = 0.50)")
    ax1.set_xlim([0.0, 1.0])
    ax1.set_ylim([0.0, 1.05])
    ax1.set_xlabel("Ложноположительная доля (FPR)", fontsize=11)
    ax1.set_ylabel("Истинноположительная доля (TPR)", fontsize=11)
    ax1.set_title("ROC-кривая скоринговой модели МСП", fontsize=12, fontweight="bold")
    ax1.legend(loc="lower right", frameon=True)
    ax1.grid(True, linestyle=":", alpha=0.6)
    
    # 2. Децильное распределение дефолта
    test_results = pd.DataFrame({"y_true": y_test, "pd": y_pred_proba})
    test_results["decile"] = pd.qcut(test_results["pd"], q=10, labels=False) + 1
    decile_stats = test_results.groupby("decile")["y_true"].mean() * 100
    
    ax2.bar(decile_stats.index, decile_stats.values, color="#EF3124", alpha=0.85, edgecolor="#B51910")
    ax2.set_xlabel("Дециль скоринга (от низкого риска к высокому)", fontsize=11)
    ax2.set_ylabel("Фактический % дефолтов в дециле", fontsize=11)
    ax2.set_title("Калибровка модели: риск по децилям", fontsize=12, fontweight="bold")
    ax2.set_xticks(range(1, 11))
    ax2.grid(axis="y", linestyle=":", alpha=0.6)
    
    plt.tight_layout()
    chart_path = docs_dir / "roc_auc_curve.png"
    plt.savefig(chart_path, dpi=200)
    plt.close()
    print(f"График валидации успешно сохранен: {chart_path}")

if __name__ == "__main__":
    train_and_evaluate()
