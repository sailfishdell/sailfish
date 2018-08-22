set term svg
set output "BASE.svg"
set xlabel "Number of concurrent requests"
set ylabel "Latency"
plot  \
    "BASE.csv" using 1:2 title "MIN" with lines, \
    "BASE.csv" using 1:3 title "MEAN" with lines, \
    "BASE.csv" using 1:4 title "MEDIAN" with lines, \
    "BASE.csv" using 1:5 title "MAX" with lines

