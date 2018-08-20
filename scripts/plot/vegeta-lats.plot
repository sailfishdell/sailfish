set term svg
set output "BASE.svg"
set xlabel "concurrent requests"
set ylabel "Latency"
plot  \
    "BASE.csv" using 1:2 title "MEAN" with lines, \
    "BASE.csv" using 1:3 title "MEDIAN" with lines, \
    "BASE.csv" using 1:4 title "95%" with lines, \
    "BASE.csv" using 1:5 title "99%" with lines, \
    "BASE.csv" using 1:6 title "MAX" with lines

