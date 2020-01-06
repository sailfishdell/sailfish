package eventservice

// This will eventually need to be more flexible

type Job func()

func CreateWorkers(queuelen int, numWorkers int) chan Job {
	jobs := make(chan Job, queuelen)
	for i := 0; i < numWorkers; i++ {
		go DoWork(jobs)
	}

	return jobs
}

func DoWork(jobs chan Job) {
	for job := range jobs {
		job()
	}
}
