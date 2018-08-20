set term svg
set output "BASE.svg"
set xlabel "concurrent requests"
set ylabel "Total CPU utilization"
plot "BASE.csv" using 1:2 title "CPU" with lines
