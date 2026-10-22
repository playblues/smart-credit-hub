"""
Генератор синтетического датасета субъектов МСП для кредитного скоринга.
Параметры и распределения согласованы со статистикой Банка России и Росстата по сектору МСП.
"""
import csv
import math
import random
from pathlib import Path

def generate_sme_dataset(n_samples: int = 8000, seed: int = 42) -> Path:
    random.seed(seed)
    output_path = Path(__file__).parent / "dataset_sme_synthetic.csv"
    
    fieldnames = [
        "company_id",
        "annual_revenue",
        "requested_amount",
        "revenue_to_loan_ratio",
        "business_age_months",
        "bki_active_loans_count",
        "bki_had_past_delinquency",
        "bki_max_overdue_days",
        "is_liquidating",
        "product_type",
        "default_12m"
    ]
    
    rows = []
    for i in range(1, n_samples + 1):
        # 1. Годовая выручка: логнормальное распределение (от ~1.5 млн до 120 млн руб)
        log_rev = random.gauss(16.5, 0.9)  # медиана ~15 млн руб
        revenue = max(1_000_000.0, min(150_000_000.0, math.exp(log_rev)))
        revenue = round(revenue, -4)
        
        # 2. Продукт и запрашиваемая сумма
        product = random.choice(["WORKING_CAPITAL", "OVERDRAFT"])
        if product == "OVERDRAFT":
            max_p = 3_000_000.0
            req_amount = random.uniform(300_000.0, max_p * 1.2)
        else:
            max_p = 10_000_000.0
            req_amount = random.uniform(1_000_000.0, max_p * 1.3)
        req_amount = round(req_amount, -4)
        
        ratio = min(revenue / req_amount, 3.0)
        
        # 3. Возраст бизнеса (в месяцах, от 3 до 140)
        age_months = int(random.triangular(3, 144, 36))
        
        # 4. БКИ данные
        active_loans = random.choices([0, 1, 2, 3, 4, 5], weights=[0.25, 0.35, 0.22, 0.12, 0.04, 0.02])[0]
        had_delinq = random.choices([0, 1], weights=[0.78, 0.22])[0]
        
        # Просрочки 90+ (стоп-фактор) и ликвидация (стоп-фактор)
        is_liquidating = 1 if random.random() < 0.015 else 0
        if had_delinq:
            max_overdue = random.choices([15, 30, 60, 95, 120], weights=[0.4, 0.3, 0.15, 0.1, 0.05])[0]
        else:
            max_overdue = 0
            
        # 5. Истинная латентная функция риска (Z-score калиброванный)
        z = (
            -2.60
            - 0.40 * ratio
            - 0.45 * (age_months / 12.0)
            + 0.40 * active_loans
            + 0.60 * had_delinq
            + (2.5 if is_liquidating else 0.0)
            + (2.0 if max_overdue >= 90 else 0.0)
            + (1.5 if age_months < 6 else 0.0)
            + random.gauss(0, 0.22)  # реалистичный уровень зашумленности кредитных данных
        )
        pd_score = 1.0 / (1.0 + math.exp(-z))
        
        # Калибровка базового уровня дефолта в секторе МСП (~4-5%)
        default_12m = 1 if random.random() < pd_score else 0
        
        rows.append({
            "company_id": f"SME_{i:06d}",
            "annual_revenue": int(revenue),
            "requested_amount": int(req_amount),
            "revenue_to_loan_ratio": round(ratio, 4),
            "business_age_months": age_months,
            "bki_active_loans_count": active_loans,
            "bki_had_past_delinquency": had_delinq,
            "bki_max_overdue_days": max_overdue,
            "is_liquidating": is_liquidating,
            "product_type": product,
            "default_12m": default_12m
        })
        
    with open(output_path, "w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)
        
    print(f"Сгенерирован датасет: {output_path} ({len(rows)} строк)")
    return output_path

if __name__ == "__main__":
    generate_sme_dataset()
