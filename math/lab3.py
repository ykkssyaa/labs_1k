import numpy as np
import matplotlib.pyplot as plt
from scipy.interpolate import interp1d


# ==============================
# 1. Система уравнений
# ==============================
def f(x, y):
    y1, y2 = y
    return np.array([y2, np.cos(3 * x) - 4 * y1])


# ==============================
# 2. Метод Рунге-Кутты 4 порядка
# ==============================
def runge_kutta(h):
    x_vals = np.arange(0, 1 + h, h)
    y = np.array([0.8, 2.0])

    y_vals = []

    for x in x_vals:
        y_vals.append(y[0])

        k1 = h * f(x, y)
        k2 = h * f(x + h / 2, y + k1 / 2)
        k3 = h * f(x + h / 2, y + k2 / 2)
        k4 = h * f(x + h, y + k3)

        y = y + (k1 + 2 * k2 + 2 * k3 + k4) / 6

    return x_vals, np.array(y_vals)


# ==============================
# 3. Вычисления для разных шагов
# ==============================
steps = [0.1, 0.01, 0.001]
results = {}

for h in steps:
    x, y = runge_kutta(h)
    results[h] = (x, y)

# ==============================
# 4. Отдельные графики решений
# ==============================
for h in steps:
    x, y = results[h]

    plt.figure()
    plt.plot(x, y)
    plt.title(f"Решение (шаг h = {h})")
    plt.xlabel("x")
    plt.ylabel("y(x)")
    plt.grid()
    plt.show()

# ==============================
# 5. Сравнение с эталоном (h=0.001)
# ==============================
x_ref, y_ref = results[0.001]
f_ref = interp1d(x_ref, y_ref, kind='cubic')

errors = []
error_functions = {}

for h in [0.1, 0.01]:
    x, y = results[h]
    y_true = f_ref(x)

    error = np.abs(y - y_true)
    max_error = np.max(error)

    errors.append((h, max_error))
    error_functions[h] = (x, error)

# ==============================
# 6. Графики ошибок
# ==============================
for h in [0.1, 0.01]:
    x, err = error_functions[h]

    plt.figure()
    plt.plot(x, err)
    plt.title(f"Ошибка (h = {h})")
    plt.xlabel("x")
    plt.ylabel("|y - y_ref|")
    plt.grid()
    plt.show()

# ==============================
# 7. Таблица ошибок и сходимость
# ==============================
print("Максимальные ошибки:")
for h, err in errors:
    print(f"h = {h}: {err:.6e}")

# оценка порядка сходимости
h1, e1 = errors[0]
h2, e2 = errors[1]

order = np.log(e1 / e2) / np.log(h1 / h2)

print(f"\nОценка порядка сходимости: p ≈ {order:.2f}")

plt.loglog([0.1,0.01], [e1,e2], 'o-')
plt.xlabel("h")
plt.ylabel("Ошибка")
plt.grid()