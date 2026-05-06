import matplotlib.pyplot as plt

# --------------------
# Параметры модели
# --------------------
steps = 1250              # количество шагов
dt = 1                   # 1 секунда

R = [120]                # начальный RPS
servers = [4]            # начальное число серверов

capacity_per_server = 80 # RPS на один сервер
Rmax = 2500              # максимальный возможный поток
r = 0.07                 # скорость роста трафика

A = 0.75                 # порог CPU (75%)
k = 1                    # сколько серверов добавляем
tau = 35                 # задержка запуска (сек)

min_servers = 4          # минимум серверов

# очередь запуска серверов
pending = []

# для графика
cluster_capacity = []

# --------------------
# Моделирование
# --------------------
for t in range(steps):

    current_servers = servers[-1]

    # запуск серверов по таймеру
    new_pending = []
    for launch_time, count in pending:
        if launch_time <= t:
            current_servers += count
        else:
            new_pending.append((launch_time, count))
    pending = new_pending

    capacity = current_servers * capacity_per_server
    cluster_capacity.append(capacity)

    load = R[-1] / capacity

    # автоскейлинг вверх
    if load > A:
        pending.append((t + tau, k))

    # автоскейлинг вниз
    if load < A and current_servers > min_servers:
        current_servers -= 1

    servers.append(current_servers)

    # логистический рост нагрузки
    next_R = R[-1] + r * R[-1] * (1 - R[-1] / Rmax)

    # искусственный всплеск
    #if 40 <= t <= 80:
    #    next_R *= 1.04

    R.append(next_R)

# --------------------
# Графики
# --------------------
plt.figure(figsize=(12, 6))
plt.plot(R[:-1], label="Входящий RPS")
plt.plot(cluster_capacity, label="Мощность кластера")
plt.xlabel("Время (сек)")
plt.ylabel("RPS")
plt.legend()
plt.grid()
plt.show()

plt.figure(figsize=(12, 4))
plt.plot(servers[:-1], label="Количество серверов")
plt.xlabel("Время (сек)")
plt.ylabel("Серверы")
plt.grid()
plt.legend()
plt.show()