goftp
=========

forked from dennisfrancis/goftp

What I've done
--------------

Fix a bug when FTP server is Serv-U FTP Server v4.0 for WinSock.

Sometimes the response has information like following:

	Response:	226-Maximum disk quota limited to 1000000 Kbytes
	Response:	    Used disk quota 0 Kbytes, available 1000000 Kbytes
	Response:	226 Transfer complete.


HOW TO
------
	wirte a MyReadCodeLine function to replace the ReadCodeLine in net/textproto
	the differences are:
	
1. ingnore the "unexpected multi-line response" err
2. when 226-Maximum disk quota limited to 1000000 Kbytes,
   just readLine() two more times