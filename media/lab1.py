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

# Преобразование изображения из RGB в XYZ
image_xyz = color.rgb2xyz(image_rgb)

# Преобразование из XYZ в Yxy
denom_xyz = image_xyz[:, :, 0] + image_xyz[:, :, 1] + image_xyz[:, :, 2]
denom_xyz[denom_xyz == 0] = 1e-10

x = image_xyz[:, :, 0] / denom_xyz
y = image_xyz[:, :, 1] / denom_xyz
Y = image_xyz[:, :, 1]

image_yxy = np.stack((Y, x, y), axis=-1)

x_red1 = 0.60
y_red1 = 0.40
x_red2 = 0.75
y_red2 = 0.20

x_WP = 0.333
y_WP = 0.333

k1 = (y_red1 - y_WP) / (x_red1 - x_WP)
k2 = (y_red2 - y_WP) / (x_red2 - x_WP)

for i in range(M):
    for j in range(N):
        if ((x[i, j] > x_WP) &
            (y[i, j] < y_WP + k1 * (x[i, j] - x_WP)) &
            (y[i, j] > y_WP + k2 * (x[i, j] - x_WP))):

            x[i, j] = x_WP
            y[i, j] = y_WP


# Преобразование из Yxy обратно в XYZ
image_xyz_new = np.zeros_like(image_xyz)
image_xyz_new[:, :, 1] = Y
image_xyz_new[:, :, 0] = x * (Y / y)
image_xyz_new[:, :, 2] = (1 - x - y) * (Y / y)

# Преобразование из XYZ обратно в RGB
image_rgb_new = color.xyz2rgb(image_xyz_new)*255
image_rgb_new = image_rgb_new.astype(np.float32)
image_bgr_new = cv2.cvtColor(image_rgb_new, cv2.COLOR_RGB2BGR)
image_rgb_new = image_rgb_new.astype(int)

plt.figure(figsize=(10, 5))
plt.subplot(1, 2, 1)
plt.title('Иcходное изображение')
plt.imshow(image_rgb)
plt.axis('off')
plt.subplot(1, 2, 2)
plt.title('Преобразованное изображение')
plt.imshow(np.clip(image_rgb_new, 0, 255))
plt.axis('off')
plt.show()
cv2.imwrite("image_out.jpg", image_bgr_new)