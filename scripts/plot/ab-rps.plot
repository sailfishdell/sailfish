set term svg
set output "BASE.svg"
set xlabel "concurrent requests"
set ylabel "Total Requests per Second"
plot "BASE.csv" using 1:2 title "RPS" with lines
