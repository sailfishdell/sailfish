set term svg
set output "BASE/RPS-WHICH.svg"
set xlabel "Requests/Second Rate"
set ylabel "Total Requests per Second"
plot "BASE/LATENCIES-WHICH.csv" using 1:7 title "RPS" with lines
