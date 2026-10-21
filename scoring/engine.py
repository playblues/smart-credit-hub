"""Эталонная реализация скоринга. Go-пакет обязан давать те же результаты (см. testdata/cases.json)."""
import json, math, pathlib

MODEL = json.loads((pathlib.Path(__file__).parent / "model.json").read_text(encoding="utf-8"))

def _get(data, path):
    for p in path.split("."):
        data = data[p]
    return data

def _cmp(a, op, b):
    return {">=": a >= b, "<": a < b, "==": a == b, ">": a > b, "<=": a <= b}[op]

def annuity_payment(amount, annual_rate, months):
    r = annual_rate / 12
    return amount * r / (1 - (1 + r) ** -months)

def pti(amount, rate, months, annual_revenue):
    return 12 * annuity_payment(amount, rate, months) / annual_revenue

def round_up_step(x, step):  # вниз до шага
    return math.floor(x / step) * step

def counter_offer(req, company, zone):
    m, co = MODEL["products"][req["product_type"]], MODEL["counter_offer_rules"]
    over_limit = req["requested_amount"] > m["max_amount"] or req["requested_term_months"] > m["max_term_months"]
    amount = min(req["requested_amount"], m["max_amount"])
    term = min(req["requested_term_months"], m["max_term_months"])
    rate = m["base_rate"]
    reason = co["reason_client_limit"]
    if zone == "YELLOW":
        amount = round_up_step(amount * co["amount_cut_multiplier"], co["amount_step"])
        rate = round(m["base_rate"] + co["rate_markup"], 4)
        reason = co["reason_client_cut"]
    while amount > co["amount_step"] and pti(amount, rate, term, company["annual_revenue"]) > co["max_pti"]:
        amount -= co["amount_step"]
        reason = co["reason_client_cut"]
    return {"amount": amount, "term_months": term, "rate": rate,
            "pti": round(pti(amount, rate, term, company["annual_revenue"]), 4), "reason": reason}, over_limit

def evaluate(inp):
    c, b, r = inp["company"], inp["bki"], inp["request"]
    ctx = {"company": c, "bki": b}
    for sf in MODEL["stop_factors"]:
        if _cmp(_get(ctx, sf["field"]), sf["op"], sf["threshold"]):
            return {"risk_zone": "RED", "status": "REJECTED", "stop_factor": sf["id"],
                    "reason_client": sf["reason_client"]}
    f = MODEL["scoring_formula"]; w = f["weights"]
    ratio = min(c["annual_revenue"] / r["requested_amount"], f["ratio_cap"])
    z = (f["intercept"] + w["revenue_to_loan_ratio"] * ratio
         + w["business_age_years"] * c["business_age_months"] / 12
         + w["bki_active_loans_count"] * b["active_loans_count"]
         + w["bki_had_past_delinquency"] * (1 if b["had_past_delinquency"] else 0))
    pd = 1 / (1 + math.exp(-z))
    zones = MODEL["risk_zones"]
    zone = "GREEN" if pd <= zones["green"]["max_pd"] else "YELLOW" if pd <= zones["yellow"]["max_pd"] \
        else "GREY" if pd <= zones["grey"]["max_pd"] else "RED"
    out = {"z_score": round(z, 3), "pd": round(pd, 4), "score": round(1000 * (1 - pd)), "risk_zone": zone}
    if zone == "RED":
        out.update(status="REJECTED", reason_client="Совокупный уровень риска превышает допустимый")
    elif zone == "GREY":
        out.update(status="MANUAL_REVIEW")
    else:
        co, over = counter_offer(r, c, zone)
        pti_req = pti(r["requested_amount"], MODEL["products"][r["product_type"]]["base_rate"],
                      r["requested_term_months"], c["annual_revenue"])
        needs_offer = zone == "YELLOW" or over or pti_req > MODEL["counter_offer_rules"]["max_pti"]
        if needs_offer:
            out.update(status="COUNTER_OFFERED", counter_offer=co)
        else:
            out.update(status="AUTO_APPROVED", approved_amount=r["requested_amount"],
                       approved_term_months=r["requested_term_months"],
                       interest_rate=MODEL["products"][r["product_type"]]["base_rate"])
    return out
