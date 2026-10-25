import numpy as np
import cv2
import matplotlib.pyplot as plt
from skimage import color


def lines_cross_point(line1, line2):
    (((x1, y1), (x2, y2)), ((x3, y3), (x4, y4))) = (line1, line2)
    (dx1, dy1) = (x1 - x2, y1 - y2)
    (dx2, dy2) = (x3 - x4, y3 - y4)
    det = dx1 * dy2 - dx2 * dy1
    if det == 0:
        return None
    det_x = (x1 * y2 - y1 * x2) * dx2 - (x3 * y4 - y3 * x4) * dx1
    det_y = (x1 * y2 - y1 * x2) * dy2 - (x3 * y4 - y3 * x4) * dy1
    return (det_x / det, det_y / det)


def angle(x, y):
    return np.angle((x - x_WP) + 1j * (y - y_WP))


# вершины треугольника цветового охвата
XYZ_R = color.rgb2xyz(np.array([[[255, 0, 0]]], dtype=np.uint8))[0, 0]
(x_R, y_R) = (XYZ_R[0] / XYZ_R.sum(), XYZ_R[1] / XYZ_R.sum())
XYZ_G = color.rgb2xyz(np.array([[[0, 255, 0]]], dtype=np.uint8))[0, 0]
(x_G, y_G) = (XYZ_G[0] / XYZ_G.sum(), XYZ_G[1] / XYZ_G.sum())
XYZ_B = color.rgb2xyz(np.array([[[0, 0, 255]]], dtype=np.uint8))[0, 0]
(x_B, y_B) = (XYZ_B[0] / XYZ_B.sum(), XYZ_B[1] / XYZ_B.sum())

image_path = input("Путь к изображению: ")
image_in = cv2.imread(image_path)

if image_in is None:
    print("Ошибка: изображение не найдено.")
    exit()

(M, N, K) = image_in.shape
image_rgb = cv2.cvtColor(image_in, cv2.COLOR_BGR2RGB)

# Преобразование из RGB в XYZ, затем в Yxy
image_xyz = color.rgb2xyz(image_rgb)
denom_xyz = image_xyz[:, :, 0] + image_xyz[:, :, 1] + image_xyz[:, :, 2]
denom_xyz[denom_xyz == 0] = 1e-10
x = image_xyz[:, :, 0] / denom_xyz
y = image_xyz[:, :, 1] / denom_xyz
Y = image_xyz[:, :, 1].copy()

(x_WP, y_WP) = (0.333, 0.333)

# границы секторов: для анализа (in) и для синтеза (out)
(x_yellow_left_in, y_yellow_left_in) = (0.39, 0.60)
(x_yellow_right_in, y_yellow_right_in) = (0.56, 0.46)
(x_yellow_left_out, y_yellow_left_out) = (0.42, 0.52)
(x_yellow_right_out, y_yellow_right_out) = (0.48, 0.48)

(x_purple_left_in, y_purple_left_in) = (0.52, 0.155)
(x_purple_right_in, y_purple_right_in) = (0.17, 0.11)
(x_purple_left_out, y_purple_left_out) = (0.35, 0.13)
(x_purple_right_out, y_purple_right_out) = (0.22, 0.10)

angle_yellow_left_in = angle(x_yellow_left_in, y_yellow_left_in)
angle_yellow_sector_in = angle(x_yellow_right_in, y_yellow_right_in) - angle_yellow_left_in
angle_yellow_left_out = angle(x_yellow_left_out, y_yellow_left_out)
angle_yellow_sector_out = angle(x_yellow_right_out, y_yellow_right_out) - angle_yellow_left_out

angle_purple_left_in = angle(x_purple_left_in, y_purple_left_in)
angle_purple_sector_in = angle(x_purple_right_in, y_purple_right_in) - angle_purple_left_in
angle_purple_left_out = angle(x_purple_left_out, y_purple_left_out)
angle_purple_sector_out = angle(x_purple_right_out, y_purple_right_out) - angle_purple_left_out

k_yellow_left = (y_yellow_left_in - y_WP) / (x_yellow_left_in - x_WP)
k_yellow_right = (y_yellow_right_in - y_WP) / (x_yellow_right_in - x_WP)
k_purple_left = (y_purple_left_in - y_WP) / (x_purple_left_in - x_WP)
k_purple_right = (y_purple_right_in - y_WP) / (x_purple_right_in - x_WP)

# пиксели, попавшие в секторы (слишком тёмные и почти серые не берём)
dist = np.sqrt((x - x_WP) ** 2 + (y - y_WP) ** 2)
is_yellow = ((x > x_WP) &
             (y < y_WP + k_yellow_left * (x - x_WP)) &
             (y > y_WP + k_yellow_right * (x - x_WP)) &
             (dist > 0.03) & (Y > 0.01))
is_purple = ((y < y_WP) &
             (y < y_WP + k_purple_left * (x - x_WP)) &
             (y < y_WP + k_purple_right * (x - x_WP)) &
             (dist > 0.03) & (Y > 0.01))

# диапазоны яркостей
Y_yellow_sorted = np.sort(Y[is_yellow])
Y_purple_sorted = np.sort(Y[is_purple])
min_Y_yellow = Y_yellow_sorted[int(0.02 * len(Y_yellow_sorted))]
max_Y_yellow = Y_yellow_sorted[int(0.98 * len(Y_yellow_sorted))]
min_Y_purple = Y_purple_sorted[int(0.02 * len(Y_purple_sorted))]
max_Y_purple = Y_purple_sorted[int(0.98 * len(Y_purple_sorted))]
delta_Y_yellow = max_Y_yellow - min_Y_yellow
delta_Y_purple = max_Y_purple - min_Y_purple

x_new = x.copy()
y_new = y.copy()
Y_new = Y.copy()

for m in range(M):
    for n in range(N):
        if is_purple[m, n]:
            # пурпурный -> жёлтый
            (x_P, y_P) = lines_cross_point(((x_WP, y_WP), (x[m, n], y[m, n])),
                                           ((x_B, y_B), (x_R, y_R)))
            distance_rel = dist[m, n] / np.sqrt((x_P - x_WP) ** 2 + (y_P - y_WP) ** 2)
            angle_rel = (angle(x[m, n], y[m, n]) - angle_purple_left_in) / angle_purple_sector_in

            output_angle = angle_yellow_left_out + angle_rel * angle_yellow_sector_out
            (x_temp, y_temp) = (x_WP + np.cos(output_angle), y_WP + np.sin(output_angle))
            (x_Y, y_Y) = lines_cross_point(((x_WP, y_WP), (x_temp, y_temp)),
                                           ((x_G, y_G), (x_R, y_R)))
            output_distance = distance_rel * np.sqrt((x_Y - x_WP) ** 2 + (y_Y - y_WP) ** 2)
            x_new[m, n] = x_WP + output_distance * np.cos(output_angle)
            y_new[m, n] = y_WP + output_distance * np.sin(output_angle)

            Y_old = min(max(Y[m, n], min_Y_purple), max_Y_purple)
            Y_new[m, n] = (Y_old - min_Y_purple) / delta_Y_purple * delta_Y_yellow + min_Y_yellow

        if is_yellow[m, n]:
            # жёлтый -> пурпурный
            (x_Y, y_Y) = lines_cross_point(((x_WP, y_WP), (x[m, n], y[m, n])),
                                           ((x_G, y_G), (x_R, y_R)))
            distance_rel = dist[m, n] / np.sqrt((x_Y - x_WP) ** 2 + (y_Y - y_WP) ** 2)
            angle_rel = (angle(x[m, n], y[m, n]) - angle_yellow_left_in) / angle_yellow_sector_in

            output_angle = angle_purple_left_out + angle_rel * angle_purple_sector_out
            (x_temp, y_temp) = (x_WP + np.cos(output_angle), y_WP + np.sin(output_angle))
            (x_P, y_P) = lines_cross_point(((x_WP, y_WP), (x_temp, y_temp)),
                                           ((x_B, y_B), (x_R, y_R)))
            output_distance = distance_rel * np.sqrt((x_P - x_WP) ** 2 + (y_P - y_WP) ** 2)
            x_new[m, n] = x_WP + output_distance * np.cos(output_angle)
            y_new[m, n] = y_WP + output_distance * np.sin(output_angle)

            Y_old = min(max(Y[m, n], min_Y_yellow), max_Y_yellow)
            Y_new[m, n] = (Y_old - min_Y_yellow) / delta_Y_yellow * delta_Y_purple + min_Y_purple

# Преобразование из Yxy обратно в XYZ и RGB
image_xyz_new = np.zeros_like(image_xyz)
image_xyz_new[:, :, 1] = Y_new
image_xyz_new[:, :, 0] = x_new * (Y_new / y_new)
image_xyz_new[:, :, 2] = (1 - x_new - y_new) * (Y_new / y_new)

image_rgb_new = color.xyz2rgb(image_xyz_new) * 255
image_rgb_new = np.clip(image_rgb_new, 0, 255).astype(np.uint8)

plt.figure(figsize=(10, 5))
plt.subplot(1, 2, 1)
plt.title('Исходное изображение')
plt.imshow(image_rgb)
plt.axis('off')
plt.subplot(1, 2, 2)
plt.title('Преобразованное изображение')
plt.imshow(image_rgb_new)
plt.axis('off')
plt.savefig("image_out_compare.png", bbox_inches='tight')
plt.show()

image_bgr_new = cv2.cvtColor(image_rgb_new, cv2.COLOR_RGB2BGR)
cv2.imwrite("image_out.jpg", image_bgr_new)
