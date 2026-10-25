import numpy as np
import cv2
import matplotlib.pyplot as plt
from skimage import color

image_path = input("Путь к изображению: ")

image_in = cv2.imread(image_path)

if image_in is None:
    print("Ошибка: изображение не найдено.")
    exit()

(M, N, K) = image_in.shape
image_rgb = cv2.cvtColor(image_in, cv2.COLOR_BGR2RGB)

# Поиск красных пикселей так же, как в лабораторной работе №1:
# переводим изображение в Yxy и проверяем, попадает ли точка (x, y)
# в сектор красного цвета
image_xyz = color.rgb2xyz(image_rgb)
denom_xyz = image_xyz[:, :, 0] + image_xyz[:, :, 1] + image_xyz[:, :, 2]
denom_xyz[denom_xyz == 0] = 1e-10
x = image_xyz[:, :, 0] / denom_xyz
y = image_xyz[:, :, 1] / denom_xyz

x_red1 = 0.60
y_red1 = 0.40
x_red2 = 0.75
y_red2 = 0.20

x_WP = 0.333
y_WP = 0.333

k1 = (y_red1 - y_WP) / (x_red1 - x_WP)
k2 = (y_red2 - y_WP) / (x_red2 - x_WP)

is_red = ((x > x_WP) &
          (y < y_WP + k1 * (x - x_WP)) &
          (y > y_WP + k2 * (x - x_WP)))

# Параметры сглаживающего фильтра
P_2 = 6  # половина ширины маски фильтра
P = 1 + 2 * P_2  # ширина маски - всегда нечетная
PSF = np.ones((P, P), np.float32) / (P ** 2)

Id = image_rgb.astype(np.float32)
R_I = Id[:, :, 0]; G_I = Id[:, :, 1]; B_I = Id[:, :, 2]
R_I_pad = np.pad(R_I, [P_2, P_2], 'symmetric')
G_I_pad = np.pad(G_I, [P_2, P_2], 'symmetric')
B_I_pad = np.pad(B_I, [P_2, P_2], 'symmetric')
R_J = R_I.copy(); G_J = G_I.copy(); B_J = B_I.copy()

# Нестационарный фильтр: для красных пикселей маска усредняющая,
# для остальных пиксель остается без изменений
for m in range(M):
    for n in range(N):
        if is_red[m, n]:
            R_temp = np.multiply(R_I_pad[m:m + P, n:n + P], PSF)
            G_temp = np.multiply(G_I_pad[m:m + P, n:n + P], PSF)
            B_temp = np.multiply(B_I_pad[m:m + P, n:n + P], PSF)
            R_J[m, n] = np.sum(R_temp)
            G_J[m, n] = np.sum(G_temp)
            B_J[m, n] = np.sum(B_temp)

image_rgb_new = np.dstack((R_J, G_J, B_J))
image_rgb_new = np.clip(image_rgb_new, 0, 255).astype(np.uint8)

plt.figure(figsize=(15, 5))
plt.subplot(1, 3, 1)
plt.title('Исходное изображение')
plt.imshow(image_rgb)
plt.axis('off')
plt.subplot(1, 3, 2)
plt.title('Красные пиксели')
plt.imshow(is_red, cmap='gray')
plt.axis('off')
plt.subplot(1, 3, 3)
plt.title('Преобразованное изображение')
plt.imshow(image_rgb_new)
plt.axis('off')
plt.savefig("image_out_compare.png", bbox_inches='tight')
plt.show()

image_bgr_new = cv2.cvtColor(image_rgb_new, cv2.COLOR_RGB2BGR)
cv2.imwrite("image_out.jpg", image_bgr_new)
