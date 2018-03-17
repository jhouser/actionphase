# -*- coding: utf-8 -*-
# Generated by Django 1.11.8 on 2018-01-16 17:43
from __future__ import unicode_literals

from django.db import migrations, models


class Migration(migrations.Migration):

    dependencies = [
        ('app', '0010_gamesignup_unique_20180113_0837'),
    ]

    operations = [
        migrations.AddField(
            model_name='character',
            name='details',
            field=models.TextField(null=True),
        ),
        migrations.AddField(
            model_name='character',
            name='status',
            field=models.CharField(choices=[('IP', 'In Progress'), ('RV', 'Ready for Review'), ('FN', 'Finished'), ('DL', 'Deleted')], default='IP', max_length=2),
        ),
    ]