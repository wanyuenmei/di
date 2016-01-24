(define Namespace "")
(define Provider "AmazonSpot")
(define MasterCount 1)
(define WorkerCount (+ 1 MasterCount))
(define AdminACL (list "local"))
(define SSHKeys (list
        "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDIpvF01Z6WgtqbF0Hl95o0rSL2jptjxLq82Y5N+pJYUmJWucrXN4L3B/ruWSZhh0LDrepC52xCuqaBLBH0dDLjtcZifUqIzn1DBNfYpxUIt5H+DKQ7HkVKEYlLzlinWTnTFPpeXsworVUxX3Ih3/zYpzcV0mI5UMoazs8/2W2Ts/IeQ0Fr2LgWhYLlO8ELuMP4ImQLVdL0rS8o5vaDdQMTdNQ+myfDmLvI9pT7v4kflbabUrLRzAgoKbK2GeQipWjGOU6QcXShBGBO6MG+sbco+qPHIUvhvExxjCL6InZvwnUfqAq3U6w/iYgSty3UeGxi3hKlAZ2R0wiv7pQbNWrN"
        "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQC97KJXlSqnEKXabanMMwUQUCS9/z0bh7ZJKmvHxVgJJ0jvGnYnJ9xf2xLWOPyugQUpsalkZROD33qMQWcMtLlb+M2pwnBW2AjPOTcCgGZ14QlmFOL2xFeKEFlcIUsHL5CxNgdacIQZ6ZBITLK8+ysg+XrMuoVQn2XCIbkHqNJ/79nya50eYyjYa58T8aTrmqRTNybLi0zpD1VJVFCqm1hinvotcavjTwS5EGmHdpfJCtfniDJb+GIx0fETIGNkh+ILPVeNVeHWcZ+O4Wz57/PtoOIVoazkOjdgnzxQxeOSWHcfi2qIJ9DEtP83DiiBwcrWBwKzH5TzgIEHfbCA2oc0eAdgPsiEBaDiWxQkWL/4uBZBgQgB7hmL1UXurevLh03372K7H1j86lhAQvIrDzskiSGrZJhNSrzNMaBv72AUQvPNeXWYp41DecsmNfKk7KozlZDxrlnHm/UIuMf0eQihABw7wBMNfMwwR23JqakJ4mvN4MKufO5NQ1Slh7EYiOG91rbBeahY6XX6u+e0Aa2XITl4JlZV+LZNmTLGuBAxoh+yBK3B27HyBiobVBDym5GHiFYGbulqc/bG8jlswF6Ztn4aVezzJVYniz3cI0h3LUcJU1Q1uXorg3Vq24Zp75piNuf0Jx8CWP6ZL48pNas61BLz+HrbWqADf0GEzFt3iw=="))

(label "Red"  (makeList WorkerCount       (docker "alpine")))
(label "Blue" (makeList (* 2 WorkerCount) (docker "alpine")))
(connect (list 1024 65535) "Red" "Blue")
(connect (list 1024 65535) "Blue" "Red")
