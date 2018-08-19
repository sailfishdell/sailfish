set term svg
set output "bench/ab-RPS.svg"
set xlabel "concurrent requests"
set ylabel "Total Requests per Second"
plot \
    "bench/odatalite-ab/RPS-token.csv"  using 1:2 title "odatalite" with lines,  \
    "bench/sailfish-apache-ab/RPS-token.csv"  using 1:2 title "sailfish-apache" with lines,  \
    "bench/sailfish-https-ab/RPS-token.csv"  using 1:2 title "sailfish" with lines, \
    "bench/sqlite-ab/RPS-token.csv"  using 1:2 title "sqlite" with lines
