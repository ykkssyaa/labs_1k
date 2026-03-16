import math


def simple_iteration_method(phi, x0, epsilon, name="Корень", debug=False):
    """
    Функция метода простой итерации.
    phi: итерационная функция x = phi(x)
    x0: начальное приближение
    epsilon: точность
    debug: флаг вывода каждого шага
    """
    print(f"\n--- Поиск: {name} ---")
    print(f"Начальное приближение: {x0:.2f}, точность: {epsilon:.4f}")
    x_prev = x0
    iterations = 0

    while True:
        iterations += 1
        try:
            x_next = phi(x_prev)
        except (ValueError, OverflowError):
            print(f"Ошибка: Выход за область определения или переполнение при x={x_prev}")
            return None

        diff = abs(x_next - x_prev)

        if debug:
            print(f"Итерация {iterations}: x = {x_next:.8f}, diff = {diff:.8f}")

        if diff < epsilon:
            print(f"{name} найден: {x_next:.5f}")
            print(f"Всего итераций: {iterations}")
            return x_next

        x_prev = x_next

        if iterations > 1000:  # Защита от бесконечного цикла
            print("Превышено максимальное количество итераций (метод расходится).")
            return None


debug = True

# 1. (lg(x+5) = cos(x), eps = 0.0001)
def f1(x):
    return math.log10(x + 5) - math.cos(x)

print("\n" + "="*30 + "\nВЫПОЛНЕНИЕ ЗАДАНИЯ 2\n" + "="*30)

phi_task1_1 = lambda x: x - 0.5 * f1(x)
phi_task1_2 = lambda x: -math.acos(math.log10(x + 5))
phi_task1_3 = lambda x: math.acos(math.log10(x + 5))
x1 = -4.25
x2 = -0.5
x3 = 0.5

eps1 = 0.0001

simple_iteration_method(phi_task1_1, x0=x1, epsilon=eps1, name="Корень 1 (Задача 1)", debug=debug)
simple_iteration_method(phi_task1_2, x0=x2, epsilon=eps1, name="Корень 2 (Задача 1)", debug=debug)
simple_iteration_method(phi_task1_3, x0=x3, epsilon=eps1, name="Корень 3 (Задача 1)", debug=debug)

# 2. (x^3 - 5x^2 + 3 = 0, eps = 0.01)

f2 = lambda x: x**3 - 5*x**2 + 3

phi_task2_root1 = lambda x: x - 0.1 * f2(x)

def phi_task2_1_2(x):
    val = 5 * x**2 - 3
    if val >= 0:
        return val ** (1/3)
    else:
        return - (abs(val) ** (1/3))

phi_task2_3 = lambda x: math.sqrt((x**3 + 3) / 5)

eps2 = 0.01

print("\n" + "="*30 + "\nВЫПОЛНЕНИЕ ЗАДАНИЯ 2\n" + "="*30)

simple_iteration_method(phi_task2_root1, x0=-0.5, epsilon=eps2, name="Корень 1 (Задача 2)", debug=debug)
simple_iteration_method(phi_task2_3, x0=0.5, epsilon=eps2, name="Корень 2 (Задача 2)", debug=debug)
simple_iteration_method(phi_task2_1_2, x0=4.75, epsilon=eps2, name="Корень 3 (Задача 2)", debug=debug)