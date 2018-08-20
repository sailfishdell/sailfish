set term svg
set output "ab-RPS.svg"
set xlabel "concurrent requests"
set ylabel "Total Requests per Second"
set autoscale
plot \
    "odatalite-ab/RPS-token.csv"  using 1:2 title "odatalite" with lines,  \
    "sailfish-apache-ab/RPS-token.csv"  using 1:2 title "sailfish-apache" with lines,  \
    "sailfish-https-ab/RPS-token.csv"  using 1:2 title "sailfish" with lines, \
    "sqlite-ab/RPS-token.csv"  using 1:2 title "sqlite" with lines
