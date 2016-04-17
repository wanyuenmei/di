(define (Println msg)
  (log "print" msg))

(define (Debugln msg)
  (log "debug" msg))

(define (Infoln msg)
  (log "info" msg))

(define (Warnln msg)
  (log "warn" msg))

(define (Errorln msg)
  (log "error" msg))
