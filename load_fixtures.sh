python manage.py sqlflush | python manage.py dbshell
directory="actionphase/app/fixtures/*"
for file in $directory
do
	python manage.py loaddata $file
done
