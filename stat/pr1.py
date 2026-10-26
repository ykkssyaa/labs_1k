import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
from scipy import stats

df = pd.read_excel("data/UN_Tourism_inbound_expenditure_12_2025.xlsx", sheet_name="Data")
reg = pd.read_excel("data/UN_Tourism_regions_composition.xlsx", header=4)

print("Исходные данные:", df.shape)
print(df.head())
print(df.tail())

# выбор показателя и года
ind = "INBD_EXPD_BPAY_TRVL_VSTR"
d = df[(df.indicator_code == ind) & (df.year == 2023)]
d = d[["reporter_area_code", "reporter_area_label", "value"]]
d.columns = ["code", "country", "x"]

# добавление региона
names = {"Africa": "Африка", "Americas": "Америка", "East Asia and the Pacific": "Азия и Тихий океан",
         "Europe": "Европа", "Middle East": "Ближний Восток", "South Asia": "Южная Азия"}
reg = reg[["area_code_M49", "UNWTO_region_label_English"]]
reg.columns = ["code", "region"]
d = d.merge(reg, on="code", how="left")
d["region"] = d["region"].map(names)
d = d.sort_values("x", ascending=False).reset_index(drop=True)
d.to_csv("results/data_2023.csv", index=False, sep=";")
print("Подготовленные данные:")
print(d.head(10))
print(d.tail(10))

x = d["x"].values
n = len(x)


mean = x.mean()
var = x.var(ddof=1)
std = np.sqrt(var)
med = np.median(x)
q1, q3 = np.percentile(x, [25, 75])
iqr = q3 - q1
cv = std / mean * 100
skew = stats.skew(x)
kurt = stats.kurtosis(x)
print(f"n = {n}")
print(f"min = {x.min():.2f}, max = {x.max():.2f}, размах = {x.max() - x.min():.2f}")
print(f"среднее = {mean:.2f}")
print(f"дисперсия = {var:.2f}, СКО = {std:.2f}")
print(f"медиана = {med:.2f}, Q1 = {q1:.2f}, Q3 = {q3:.2f}, IQR = {iqr:.2f}")
print(f"коэф. вариации = {cv:.1f} %")
print(f"асимметрия = {skew:.3f}, эксцесс = {kurt:.3f}")

# мода по интервальному ряду
k = int(1 + 3.322 * np.log10(n))
cnt, edges = np.histogram(x, bins=k)
i = np.argmax(cnt)
h = edges[1] - edges[0]
f0 = cnt[i]
f1 = cnt[i - 1] if i > 0 else 0
f2 = cnt[i + 1] if i < k - 1 else 0
mode = edges[i] + h * (f0 - f1) / ((f0 - f1) + (f0 - f2))
print(f"k = {k}, h = {h:.2f}, мода = {mode:.2f}")

# выбросы по правилу 1.5 IQR
up = q3 + 1.5 * iqr
out = d[d.x > up]
print(f"верхняя граница = {up:.2f}, выбросов: {len(out)}")
print(out[["country", "x"]])

plt.figure(figsize=(9, 5))
top = d.head(15)
ru = ["США", "Испания", "Великобритания", "Франция", "Италия", "Турция", "Австралия", "Канада",
      "Япония", "Германия", "Саудовская Аравия", "Индия", "Макао (Китай)", "Мексика", "Таиланд"]
plt.bar(ru, top.x / 1000, color="tab:blue", label="2023 г.")
plt.xticks(rotation=60, ha="right")
plt.ylabel("Поступления, млрд долл. США")
plt.xlabel("Страна")
plt.title("15 стран с наибольшими поступлениями от туризма")
plt.legend()
plt.grid(axis="y", alpha=0.3)
plt.tight_layout()
plt.savefig("results/top15.png", dpi=150)

plt.figure(figsize=(8, 5))
s = d.groupby("region").x.sum().sort_values(ascending=False)
plt.pie(s, autopct="%1.1f%%", startangle=90, pctdistance=1.15)
plt.legend(s.index, loc="center left", bbox_to_anchor=(1, 0.5))
plt.title("Доля регионов в сумме поступлений, 2023 г.")
plt.tight_layout()
plt.savefig("results/pie.png", dpi=150)

plt.figure(figsize=(8, 5))
plt.hist(x / 1000, bins=k, edgecolor="black", label="частоты")
plt.xlabel("Поступления, млрд долл. США")
plt.ylabel("Количество стран")
plt.title("Гистограмма исходных данных")
plt.legend()
plt.grid(alpha=0.3)
plt.tight_layout()
plt.savefig("results/hist.png", dpi=150)


def chi2_test(z, edges, n_par):
    o, _ = np.histogram(z, bins=edges)
    e = np.full(k, n / k)
    chi = ((o - e) ** 2 / e).sum()
    r = k - 1 - n_par
    crit = stats.chi2.ppf(0.95, r)
    return chi, r, crit, o, e


# гипотеза 1: показательное распределение
lam = 1 / mean
edges_e = stats.expon.ppf(np.linspace(0, 1, k + 1), scale=mean)
chi_e, r_e, crit_e, o_e, e_e = chi2_test(x, edges_e, 1)
print(f"Показательное: lambda = {lam:.8f}, chi2 = {chi_e:.2f}, r = {r_e}, chi2кр = {crit_e:.2f}")
pd.DataFrame({"a": edges_e[:-1], "b": edges_e[1:], "n_i": o_e, "np_i": e_e}).to_csv(
    "results/chi2_expon.csv", index=False, sep=";")
print("границы:", np.round(edges_e, 1), "n_i:", o_e)

# гипотеза 2: логнормальное
y = np.log(x)
my, sy = y.mean(), y.std(ddof=1)
edges_y = stats.norm.ppf(np.linspace(0, 1, k + 1), my, sy)
chi_y, r_y, crit_y, o_y, e_y = chi2_test(y, edges_y, 2)
print(f"ln X: среднее = {my:.4f}, СКО = {sy:.4f}, асимметрия = {stats.skew(y):.3f}")
print(f"Нормальное для ln X: chi2 = {chi_y:.2f}, r = {r_y}, chi2кр = {crit_y:.2f}, "
      f"p-value = {stats.chi2.sf(chi_y, r_y):.3f}")
tab = pd.DataFrame({"a": edges_y[:-1], "b": edges_y[1:], "n_i": o_y, "np_i": e_y})
tab.to_csv("results/chi2_lnx.csv", index=False, sep=";")
print(tab)

plt.figure(figsize=(8, 5))
plt.hist(y, bins=k, density=True, edgecolor="black", label="гистограмма ln X")
t = np.linspace(y.min() - 1, y.max() + 1, 300)
plt.plot(t, stats.norm.pdf(t, my, sy), "r", lw=2, label="плотность N(a, σ)")
plt.xlabel("ln X (X в млн долл. США)")
plt.ylabel("Плотность")
plt.title("Гистограмма логарифмов и нормальная плотность")
plt.legend()
plt.grid(alpha=0.3)
plt.tight_layout()
plt.savefig("results/hist_ln.png", dpi=150)

plt.figure(figsize=(8, 5))
plt.hist(x, bins=k, density=True, edgecolor="black", label="гистограмма X")
t = np.linspace(1, x.max(), 500)
plt.plot(t, stats.expon.pdf(t, scale=mean), "g", lw=2, label="показательная плотность")
plt.plot(t, stats.lognorm.pdf(t, sy, scale=np.exp(my)), "r", lw=2, label="логнормальная плотность")
plt.xlabel("Поступления, млн долл. США")
plt.ylabel("Плотность")
plt.title("Гистограмма и теоретические плотности")
plt.ylim(0, 1.5 * cnt[0] / n / h)
plt.legend()
plt.grid(alpha=0.3)
plt.tight_layout()
plt.savefig("results/hist_fit.png", dpi=150)

# доверительные интервалы
tq = stats.t.ppf(0.975, n - 1)
m1, m2 = mean - tq * std / np.sqrt(n), mean + tq * std / np.sqrt(n)
c1, c2 = stats.chi2.ppf(0.975, n - 1), stats.chi2.ppf(0.025, n - 1)
v1, v2 = (n - 1) * var / c1, (n - 1) * var / c2
print(f"t = {tq:.4f}, chi2(0.975) = {c1:.2f}, chi2(0.025) = {c2:.2f}")
print(f"ДИ среднего: ({m1:.2f}; {m2:.2f})")
print(f"ДИ дисперсии: ({v1:.2f}; {v2:.2f})")
print(f"ДИ СКО: ({np.sqrt(v1):.2f}; {np.sqrt(v2):.2f})")
y1, y2 = my - tq * sy / np.sqrt(n), my + tq * sy / np.sqrt(n)
print(f"ДИ среднего ln X: ({y1:.4f}; {y2:.4f}), геом. среднее = {np.exp(my):.2f}, "
      f"ДИ: ({np.exp(y1):.2f}; {np.exp(y2):.2f})")

# анализ регионов
print(d.groupby("region").x.agg(["count", "sum", "mean", "median", "std"]).round(1))
order = d.groupby("region").x.median().sort_values(ascending=False).index
data_r = [np.log10(d[d.region == g].x) for g in order]

fig, ax = plt.subplots(2, 1, figsize=(10, 9))
ax[0].boxplot(data_r, tick_labels=order)
ax[0].set_title("Boxplot")
ax[1].violinplot(data_r, showmedians=True)
ax[1].set_xticks(range(1, len(order) + 1), order)
ax[1].set_title("Скрипичная диаграмма")
for a in ax:
    a.tick_params(axis="x", rotation=15)
    a.set_ylabel("lg X (X в млн долл. США)")
    a.grid(alpha=0.3)
plt.tight_layout()
plt.savefig("results/box_regions.png", dpi=150)

plt.figure(figsize=(8, 3))
plt.boxplot(x / 1000, orientation="horizontal")
plt.xlabel("Поступления, млрд долл. США")
plt.title("Boxplot исходных данных")
plt.grid(alpha=0.3)
plt.tight_layout()
plt.savefig("results/box.png", dpi=150)

# сравнение с 2019 годом
d19 = df[(df.indicator_code == ind) & (df.year == 2019)][["reporter_area_code", "value"]]
d19.columns = ["code", "x19"]
both = d.merge(d19, on="code")
print("стран с данными за оба года:", len(both))
for col in ["x19", "x"]:
    z = both[col]
    print(col, f"сумма = {z.sum():.0f}, среднее = {z.mean():.2f}, медиана = {z.median():.2f}, "
          f"СКО = {z.std():.2f}, CV = {z.std() / z.mean() * 100:.1f} %, асимм. = {stats.skew(z):.3f}")
both["ratio"] = both.x / both.x19 * 100
print(f"восстановились до уровня 2019 (>=100 %): {(both.ratio >= 100).sum()} из {len(both)}")
print(f"медиана отношения 2023/2019 = {both.ratio.median():.1f} %")
both.to_csv("results/compare_2019.csv", index=False, sep=";")

plt.figure(figsize=(6, 5))
plt.boxplot([np.log10(both.x19), np.log10(both.x)], tick_labels=["2019", "2023"])
plt.ylabel("lg X (X в млн долл. США)")
plt.title("Сравнение 2019 и 2023 годов")
plt.grid(alpha=0.3)
plt.tight_layout()
plt.savefig("results/box_2019.png", dpi=150)
