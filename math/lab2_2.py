import numpy as np

def f(points):
    x, y = points
    f1 = np.sin(x - 1) + y - 1.3
    f2 = x - np.sin(y + 1) - 0.8
    return np.array([f1, f2])

def jacobian(points):
    x, y = points
    return np.array([
        [np.cos(x - 1), 1],
        [1, -np.cos(y + 1)]
    ])

def newton_with_details(initial_guess, tolerance=1e-7):
    x_curr = np.array(initial_guess, dtype=float)

    print(f"{'Итер.':<7} | {'x':<10} | {'y':<10} | {'delta':<12}")
    print("-" * 50)

    for i in range(1, 20): 
        F = f(x_curr)
        J = jacobian(x_curr)

        delta = np.linalg.solve(J, -F)
        x_curr = x_curr + delta

        error = np.linalg.norm(delta)

        print(f"{i:<7} | {x_curr[0]:.7f} | {x_curr[1]:.7f} | {error:.2e}")

        if error < tolerance:
            print("-" * 50)
            print(f"Корень уравнения: x = {x_curr[0]:.5f}, y = {x_curr[1]:.5f}")
            return x_curr

    return x_curr

# Запуск с начальной точкой (1, 1)
newton_with_details([1.0, 1.0])