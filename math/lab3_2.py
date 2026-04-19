import numpy as np
import matplotlib.pyplot as plt

# ======================================================
# ЗАДАЧА:
# y' = sin(x) - 2y
# y(0) = 0.5
# Метод Рунге-Кутты 4 порядка с адаптивным шагом
# ======================================================


def f(x, y):
    return np.sin(x) - 2*y


def rk4_step(x, y, h):
    k1 = h * f(x, y)
    k2 = h * f(x + h/2, y + k1/2)
    k3 = h * f(x + h/2, y + k2/2)
    k4 = h * f(x + h, y + k3)

    return y + (k1 + 2*k2 + 2*k3 + k4) / 6


def adaptive_rk4(x0, y0, x_end, h0=0.1, tol=1e-6):
    x = x0
    y = y0
    h = h0

    xs = [x]
    ys = [y]
    hs = []

    while x < x_end:

        if x + h > x_end:
            h = x_end - x

        # Один большой шаг
        y_big = rk4_step(x, y, h)

        # Два маленьких шага
        y_half = rk4_step(x, y, h/2)
        y_small = rk4_step(x + h/2, y_half, h/2)

        # Оценка ошибки
        error = abs(y_small - y_big) / 15

        # Если шаг хороший — принимаем
        if error < tol:
            x += h
            y = y_small

            xs.append(x)
            ys.append(y)
            hs.append(h)

        # Новый шаг
        if error == 0:
            h = h * 2
        else:
            h = 0.9 * h * (tol / error)**0.2

        # Ограничения шага
        h = min(h, 0.2)
        h = max(h, 1e-4)

    return np.array(xs), np.array(ys), np.array(hs)


x0 = 0
y0 = 0.5
x_end = 5

x_vals, y_vals, h_vals = adaptive_rk4(x0, y0, x_end)


plt.figure(figsize=(10,5))
plt.plot(x_vals, y_vals, marker='o', markersize=3)
plt.title("Приближенное решение y(x)")
plt.xlabel("x")
plt.ylabel("y")
plt.grid()
plt.show()


plt.figure(figsize=(10,5))
plt.plot(x_vals[:-1], h_vals, marker='o', markersize=3)
plt.title("Зависимость шага h от x")
plt.xlabel("x")
plt.ylabel("h")
plt.grid()
plt.show()

print("       x           y(x)")
print("---------------------------")

for i in range(len(x_vals)):
    print(f"{x_vals[i]:10.5f}   {y_vals[i]:12.8f}")

print("\nКоличество шагов:", len(h_vals))
print("Минимальный шаг:", np.min(h_vals))
print("Максимальный шаг:", np.max(h_vals))